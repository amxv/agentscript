package transcript

import (
	"bytes"
	"encoding/json"
	"strings"
)

func isAntigravityCapture(obj map[string]any) bool {
	if obj["conversation_id"] != nil && (obj["status"] != nil && obj["duration_seconds"] != nil || obj["event"] == "init" && obj["init"] != nil) {
		return true
	}
	if step := mapValue(obj["step_update"]); obj["event"] == "step_update" && step["conversation_id"] != nil {
		return true
	}
	if result := mapValue(obj["result"]); obj["event"] == "result" && result["conversation_id"] != nil {
		return true
	}
	msg := mapValue(obj["msg"])
	result := mapValue(msg["result"])
	params := mapValue(msg["params"])
	name := stringValue(mapValue(result["agentInfo"])["name"])
	return strings.HasPrefix(name, "Antigravity") || mapValue(params["_meta"])["antigravity"] != nil || mapValue(result["_meta"])["antigravity"] != nil
}

type captureSession struct {
	turn       int
	chunk      int
	chunkID    string
	reply      bool
	text       strings.Builder
	promptEcho string
}

type captureRequest struct {
	session, method    string
	turn, previousTurn int
}

type agyCapture struct {
	blocks   []Block
	sessions map[string]*captureSession
	calls    map[string]Block
	results  map[string]bool
	requests map[string]captureRequest
	nextTurn int
}

func parseAntigravityCapture(data []byte) ([]Block, error) {
	c := agyCapture{sessions: map[string]*captureSession{}, calls: map[string]Block{}, results: map[string]bool{}, requests: map[string]captureRequest{}}
	// A --output-format json envelope may span several physical lines.
	if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] == '{' && json.Valid(trimmed) {
		c.record(rawObject(trimmed))
		return c.blocks, nil
	}
	err := scanJSONLines(data, func(e jsonLine) { c.record(rawObject(e.Raw)) })
	return c.blocks, err
}

func (c *agyCapture) session(id string) *captureSession {
	if c.sessions[id] == nil {
		c.sessions[id] = &captureSession{chunk: -1}
	}
	return c.sessions[id]
}

func (c *agyCapture) startTurn(s *captureSession) {
	c.nextTurn++
	s.turn = c.nextTurn
	s.chunk = -1
	s.reply = false
}

func (c *agyCapture) add(s *captureSession, b Block) {
	b.Turn = s.turn
	c.blocks = append(c.blocks, b)
	s.chunk = -1
}

func (c *agyCapture) chunk(s *captureSession, kind Kind, id, text string) {
	if text == "" {
		return
	}
	if s.chunk >= 0 && c.blocks[s.chunk].Kind == kind && s.chunkID == id {
		s.text.WriteString(text)
		c.blocks[s.chunk].Text = s.text.String()
		return
	}
	s.text.Reset()
	s.text.WriteString(text)
	role := "assistant"
	if kind == KindUser {
		role = "user"
		c.startTurn(s)
	}
	c.add(s, Block{Kind: kind, Role: role, Text: text})
	s.chunk = len(c.blocks) - 1
	s.chunkID = id
	if kind == KindAssistant {
		s.reply = true
	}
}

func agyContent(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		var lines []string
		for _, part := range v {
			if text := agyContent(part); text != "" {
				lines = append(lines, text)
			}
		}
		return strings.Join(lines, "\n")
	case map[string]any:
		switch v["type"] {
		case "image":
			return imageAttachment
		case "audio":
			return "[Audio attachment]"
		case "diff":
			return strings.TrimSpace(stringValue(v["path"]) + "\n" + stringValue(v["oldText"]) + "\n" + stringValue(v["newText"]))
		case "resource":
			return agyContent(v["resource"])
		case "resource_link":
			return stringValue(v["uri"])
		case "content":
			return agyContent(v["content"])
		}
		if text, ok := v["text"].(string); ok {
			return text
		}
		return stringValue(v["uri"])
	}
	return ""
}

func (c *agyCapture) record(obj map[string]any) {
	if msg := mapValue(obj["msg"]); msg != nil {
		c.acp(msg, stringValue(obj["dir"]))
		return
	}
	switch obj["event"] {
	case "init":
		c.session(stringValue(obj["conversation_id"])).reply = false
	case "step_update":
		c.printStep(mapValue(obj["step_update"]))
	case "result":
		c.printResult(mapValue(obj["result"]))
	default:
		if obj["status"] != nil {
			c.printResult(obj)
		}
	}
}

func (c *agyCapture) printResult(obj map[string]any) {
	s := c.session(stringValue(obj["conversation_id"]))
	if !s.reply {
		response := stringValue(obj["response"])
		if response == "" {
			response = stringValue(obj["structured_output"])
		}
		c.chunk(s, KindAssistant, "", response)
	}
	for _, action := range anyList(obj["denied_actions"]) {
		c.add(s, Block{Kind: KindSystem, Role: "system", Text: "Permission denied: " + stringValue(mapValue(action)["display_name"]), IsError: true})
	}
	if status := stringValue(obj["status"]); status != "" && status != "SUCCESS" {
		c.add(s, Block{Kind: KindSystem, Role: "system", Text: strings.TrimSpace(status + " " + stringValue(obj["error"])), IsError: true})
	}
	s.chunk = -1
	s.reply = false
}

func anyList(v any) []any {
	list, _ := v.([]any)
	return list
}

func (c *agyCapture) printStep(step map[string]any) {
	sid := stringValue(step["conversation_id"])
	s := c.session(sid)
	id := sid + "/step-" + stringValue(step["step_index"])
	text := stringValue(step["text_delta"])
	kind := stringValue(step["step_type"])
	if step["subagent_info"] != nil {
		kind = "subagent"
	}
	switch kind {
	case "user_input":
		c.startTurn(s)
		if text != "" {
			c.add(s, Block{Kind: KindUser, Role: "user", Text: text})
		}
	case "agent_response":
		c.chunk(s, KindAssistant, id, text)
	case "tool", "subagent":
		info := mapValue(step["tool_info"])
		name := stringValue(step["tool_name"])
		if name == "" {
			name = stringValue(info["name"])
		}
		if name == "" && step["subagent_info"] != nil {
			name = "invoke_subagent"
		}
		c.tool(s, id, name, mapValue(info["parameters"]))
		if state := stringValue(step["state"]); state == "DONE" || state == "ERROR" || state == "CANCELED" || state == "INTERRUPTED" {
			output := agyResultText(stringValue(info["output"]))
			if subagents := step["subagent_info"]; subagents != nil {
				output = strings.TrimSpace(output + "\n" + stringValue(subagents))
			}
			detail := stringValue(mapValue(info["error"])["message"])
			if detail == "" {
				detail = stringValue(step["error"])
			}
			if detail != "" && !strings.Contains(output, detail) {
				output = strings.TrimSpace(output + "\n" + detail)
			}
			c.result(s, id, output, agyToolFailed(state, detail, nil, c.calls[id].Kind, output))
		}
	default:
		if text != "" {
			c.add(s, Block{Kind: KindSystem, Role: "system", Text: text})
		}
	}
}

func (c *agyCapture) tool(s *captureSession, id, name string, args map[string]any) {
	if _, ok := c.calls[id]; ok {
		return
	}
	call := agyToolCall(name, id, args)
	c.calls[id] = call
	c.add(s, call)
}

func (c *agyCapture) result(s *captureSession, id, output string, failed bool) {
	if c.results[id] {
		return
	}
	c.results[id] = true
	call := c.calls[id]
	c.add(s, resultBlockFor(id, pendingTool{Name: call.ToolName, Kind: call.Kind}, output, "", failed, nil))
}

func (c *agyCapture) acp(msg map[string]any, dir string) {
	params := mapValue(msg["params"])
	sid := stringValue(params["sessionId"])
	s := c.session(sid)
	rid := stringValue(msg["id"])
	method := stringValue(msg["method"])
	if method != "" && rid != "" {
		previous := s.turn
		if method == "session/prompt" {
			c.startTurn(s)
		}
		c.requests[dir+rid] = captureRequest{session: sid, method: method, turn: s.turn, previousTurn: previous}
	}
	switch method {
	case "session/prompt":
		s.promptEcho = agyContent(params["prompt"])
		c.add(s, Block{Kind: KindUser, Role: "user", Text: s.promptEcho})
	case "session/update":
		u := mapValue(params["update"])
		text := agyContent(u["content"])
		id := stringValue(u["messageId"])
		switch u["sessionUpdate"] {
		case "user_message_chunk":
			if s.promptEcho != "" && s.promptEcho == text {
				s.promptEcho = ""
				return
			}
			c.chunk(s, KindUser, id, text)
		case "agent_message_chunk":
			c.chunk(s, KindAssistant, id, text)
		case "agent_thought_chunk":
			c.chunk(s, KindThinking, id, text)
		case "tool_call", "tool_call_update":
			c.acpTool(s, sid, u)
		case "compaction_summary_chunk", "notice", "plan", "plan_update", "session_message", "session_message_chunk":
			if text == "" {
				text = stringValue(u["message"])
			}
			if text == "" {
				text = stringValue(u["entries"])
			}
			c.add(s, Block{Kind: KindSystem, Role: "system", Text: text})
		}
	case "session/request_permission":
		u := mapValue(params["toolCall"])
		c.acpTool(s, sid, u)
		c.add(s, Block{Kind: KindSystem, Role: "system", Text: "Permission requested: " + stringValue(u["title"])})
	case "elicitation/create":
		c.add(s, Block{Kind: KindSystem, Role: "system", Text: strings.TrimSpace(stringValue(params["message"]) + "\n" + stringValue(params["requestedSchema"]))})
	case "":
		peer := "out"
		if dir == "out" {
			peer = "in"
		}
		request := c.requests[peer+rid]
		delete(c.requests, peer+rid)
		s = c.session(request.session)
		res := mapValue(msg["result"])
		if detail := stringValue(mapValue(msg["error"])["message"]); detail != "" {
			c.add(s, Block{Kind: KindSystem, Role: "system", Text: detail, IsError: true})
			c.blocks[len(c.blocks)-1].Turn = request.turn
			if request.method == "session/prompt" && request.turn == s.turn {
				s.turn = request.previousTurn
				s.promptEcho = ""
			}
			return
		}
		switch request.method {
		case "session/prompt":
			s.chunk = -1
			if reason := stringValue(res["stopReason"]); reason != "" && reason != "end_turn" {
				c.add(s, Block{Kind: KindSystem, Role: "system", Text: reason})
				c.blocks[len(c.blocks)-1].Turn = request.turn
			}
		case "session/request_permission", "elicitation/create":
			c.add(s, Block{Kind: KindSystem, Role: "system", Text: stringValue(res)})
		}
	}
}

func (c *agyCapture) acpTool(s *captureSession, sid string, u map[string]any) {
	id := sid + "/" + stringValue(u["toolCallId"])
	args := mapValue(u["rawInput"])
	if args == nil {
		args = mapValue(u["args"])
	}
	c.tool(s, id, stringValue(u["name"]), args)
	status := stringValue(u["status"])
	if status != "completed" && status != "failed" {
		return
	}
	raw := mapValue(u["rawOutput"])
	output := stringValue(raw["output"])
	if output == "" {
		output = agyContent(u["content"])
	}
	if output == "" {
		if str, ok := u["rawOutput"].(string); ok {
			output = str
		} else if raw != nil && raw["output"] == nil && len(raw) > 0 {
			output = stringValue(raw)
		}
	}
	output = agyResultText(output)
	exit := int64Pointer(raw["exit_code"])
	c.result(s, id, output, agyToolFailed(status, "", nil, c.calls[id].Kind, output) || exit != nil && *exit != 0)
}
