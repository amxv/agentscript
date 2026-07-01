package transcript

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

type jsonLine struct {
	Type      string          `json:"type"`
	Timestamp string          `json:"timestamp"`
	Message   *message        `json:"message"`
	Payload   json.RawMessage `json:"payload"`
	Item      json.RawMessage `json:"item"`
	Raw       json.RawMessage `json:"-"`
}

type message struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type contentBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Input     map[string]any  `json:"input"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
	IsError   bool            `json:"is_error"`
}

type pendingTool struct {
	Name string
	Kind Kind
}

func ParseFile(path string) (Transcript, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Transcript{}, err
	}
	tr, err := Parse(data)
	if err != nil {
		return Transcript{}, err
	}
	tr.Path = path
	return tr, nil
}

func Parse(data []byte) (Transcript, error) {
	provider := Detect(data)
	switch provider {
	case ProviderClaude:
		blocks, err := parseClaude(data)
		return finish(provider, blocks), err
	case ProviderCodex:
		blocks, err := parseCodex(data)
		return finish(provider, blocks), err
	default:
		return Transcript{Provider: ProviderUnknown}, fmt.Errorf("unknown transcript format")
	}
}

func finish(provider Provider, blocks []Block) Transcript {
	out := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		if strings.TrimSpace(b.Text) == "" && b.Kind != KindToolCall && b.Kind != KindCommand {
			continue
		}
		b.Provider = provider
		b.Index = len(out)
		out = append(out, b)
	}
	return Transcript{Provider: provider, Blocks: out}
}

func Detect(data []byte) Provider {
	s := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 1024*1024)
	s.Buffer(buf, 32*1024*1024)
	for s.Scan() {
		line := bytes.TrimSpace(s.Bytes())
		if len(line) == 0 {
			continue
		}
		var obj map[string]any
		if json.Unmarshal(line, &obj) != nil {
			continue
		}
		if typ, _ := obj["type"].(string); typ != "" {
			switch typ {
			case "session_meta", "thread.started":
				return ProviderCodex
			case "item.completed":
				if _, ok := obj["item"]; ok {
					return ProviderCodex
				}
			case "user", "assistant":
				return ProviderClaude
			}
		}
	}
	return ProviderUnknown
}

func parseClaude(data []byte) ([]Block, error) {
	entries, err := readJSONLines(data)
	if err != nil {
		return nil, err
	}
	var blocks []Block
	pending := map[string]pendingTool{}
	seenToolIDs := map[string]bool{}

	for _, entry := range entries {
		if entry.Type != "user" && entry.Type != "assistant" {
			continue
		}
		role := entry.Type
		if entry.Message != nil && entry.Message.Role != "" {
			role = entry.Message.Role
		}
		if entry.Message == nil {
			continue
		}
		switch role {
		case "user":
			c := entry.Message.Content
			if isJSONString(c) {
				text := cleanSystemTags(readJSONString(c))
				if text != "" {
					blocks = append(blocks, Block{Kind: KindUser, Role: "user", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
				}
				continue
			}
			arr := decodeContentArray(c)
			var textParts []string
			for _, cb := range arr {
				switch cb.Type {
				case "tool_result":
					text := toolResultText(cb.Content)
					pt := pending[cb.ToolUseID]
					kind := KindToolResult
					if pt.Kind == KindCommand {
						kind = KindCommandResult
					}
					blocks = append(blocks, Block{Kind: kind, Role: "tool", Text: strings.TrimSpace(text), ToolID: cb.ToolUseID, ToolName: pt.Name, Timestamp: entry.Timestamp, IsError: cb.IsError, Raw: entry.Raw})
				case "text":
					if strings.TrimSpace(cb.Text) != "" {
						textParts = append(textParts, cb.Text)
					}
				}
			}
			if len(textParts) > 0 {
				text := cleanSystemTags(strings.Join(textParts, "\n"))
				if text != "" {
					blocks = append(blocks, Block{Kind: KindUser, Role: "user", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
				}
			}
		case "assistant":
			arr := decodeContentArray(entry.Message.Content)
			for _, cb := range arr {
				switch cb.Type {
				case "text":
					text := strings.TrimSpace(cb.Text)
					if text != "" && text != "No response requested." {
						blocks = append(blocks, Block{Kind: KindAssistant, Role: "assistant", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
					}
				case "thinking":
					text := strings.TrimSpace(cb.Thinking)
					if text != "" {
						blocks = append(blocks, Block{Kind: KindThinking, Role: "assistant", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
					}
				case "tool_use":
					if cb.ID != "" && seenToolIDs[cb.ID] {
						continue
					}
					seenToolIDs[cb.ID] = true
					kind := toolCallKind(cb.Name)
					input := cb.Input
					if input == nil {
						input = map[string]any{}
					}
					blocks = append(blocks, Block{Kind: kind, Role: "assistant", ToolID: cb.ID, ToolName: cb.Name, ToolInput: input, Timestamp: entry.Timestamp, Raw: entry.Raw})
					pending[cb.ID] = pendingTool{Name: cb.Name, Kind: kind}
				}
			}
		}
	}
	return blocks, nil
}

func parseCodex(data []byte) ([]Block, error) {
	entries, err := readJSONLines(data)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.Type == "thread.started" || (entry.Type == "item.completed" && len(entry.Item) > 0) {
			return parseCodexNew(entries), nil
		}
	}
	return parseCodexLegacy(entries), nil
}

func parseCodexNew(entries []jsonLine) []Block {
	var blocks []Block
	pending := map[string]pendingTool{}
	for _, entry := range entries {
		if entry.Type != "item.completed" || len(entry.Item) == 0 {
			continue
		}
		var item map[string]any
		if json.Unmarshal(entry.Item, &item) != nil {
			continue
		}
		itemType := stringValue(item["type"])
		switch itemType {
		case "message":
			role := stringValue(item["role"])
			text := textFromCodexContent(item["content"], role == "user")
			if text == "" {
				continue
			}
			if role == "user" {
				blocks = append(blocks, Block{Kind: KindUser, Role: "user", Text: extractCodexUserText(text), Timestamp: entry.Timestamp, Raw: entry.Raw})
			} else if role == "assistant" {
				blocks = append(blocks, Block{Kind: KindAssistant, Role: "assistant", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
			}
		case "agent_message":
			text := strings.TrimSpace(stringValue(item["text"]))
			if text != "" {
				blocks = append(blocks, Block{Kind: KindAssistant, Role: "assistant", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
			}
		case "reasoning":
			text := reasoningText(item)
			if text != "" {
				blocks = append(blocks, Block{Kind: KindThinking, Role: "assistant", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
			}
		case "command_execution":
			id := stringValue(item["id"])
			cmd := cleanShellCommand(stringValue(item["command"]))
			blocks = append(blocks, Block{Kind: KindCommand, Role: "assistant", ToolID: id, ToolName: "Bash", ToolInput: map[string]any{"command": cmd}, Timestamp: entry.Timestamp, Raw: entry.Raw})
			out := strings.TrimSpace(stringValue(item["aggregated_output"]))
			if out != "" {
				blocks = append(blocks, Block{Kind: KindCommandResult, Role: "tool", ToolID: id, ToolName: "Bash", Text: out, Timestamp: entry.Timestamp, IsError: numberValue(item["exit_code"]) != 0, Raw: entry.Raw})
			}
		case "function_call":
			id := stringValue(item["id"])
			if id == "" {
				id = stringValue(item["call_id"])
			}
			name := stringValue(item["name"])
			args := stringValue(item["arguments"])
			kind, displayName, input := codexCallShape(name, args)
			blocks = append(blocks, Block{Kind: kind, Role: "assistant", ToolID: id, ToolName: displayName, ToolInput: input, Timestamp: entry.Timestamp, IsError: stringValue(item["status"]) == "failed", Raw: entry.Raw})
			pending[id] = pendingTool{Name: displayName, Kind: kind}
			out := strings.TrimSpace(stringValue(item["output"]))
			if out != "" {
				blocks = append(blocks, resultBlockFor(id, pending[id], out, entry.Timestamp, stringValue(item["status"]) == "failed", entry.Raw))
			}
		}
	}
	return blocks
}

func parseCodexLegacy(entries []jsonLine) []Block {
	var blocks []Block
	pending := map[string]pendingTool{}
	inTurn := false
	for _, entry := range entries {
		switch entry.Type {
		case "event_msg":
			var payload map[string]any
			if json.Unmarshal(entry.Payload, &payload) != nil {
				continue
			}
			switch stringValue(payload["type"]) {
			case "task_started":
				inTurn = true
			case "task_complete":
				inTurn = false
			case "user_message":
				text := extractCodexUserText(stringValue(payload["message"]))
				if text != "" {
					blocks = append(blocks, Block{Kind: KindUser, Role: "user", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
				}
			}
		case "response_item":
			if !inTurn {
				continue
			}
			var payload map[string]any
			if json.Unmarshal(entry.Payload, &payload) != nil {
				continue
			}
			ptype := stringValue(payload["type"])
			switch ptype {
			case "message":
				role := stringValue(payload["role"])
				if role == "developer" {
					continue
				}
				text := textFromCodexContent(payload["content"], role == "user")
				if text == "" {
					continue
				}
				if role == "user" {
					blocks = append(blocks, Block{Kind: KindUser, Role: "user", Text: extractCodexUserText(text), Timestamp: entry.Timestamp, Raw: entry.Raw})
				} else if role == "assistant" {
					kind := KindAssistant
					if stringValue(payload["phase"]) == "commentary" {
						kind = KindThinking
					}
					blocks = append(blocks, Block{Kind: kind, Role: "assistant", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
				}
			case "reasoning":
				text := reasoningText(payload)
				if text != "" {
					blocks = append(blocks, Block{Kind: KindThinking, Role: "assistant", Text: text, Timestamp: entry.Timestamp, Raw: entry.Raw})
				}
			case "function_call":
				id := stringValue(payload["call_id"])
				kind, displayName, input := codexCallShape(stringValue(payload["name"]), stringValue(payload["arguments"]))
				blocks = append(blocks, Block{Kind: kind, Role: "assistant", ToolID: id, ToolName: displayName, ToolInput: input, Timestamp: entry.Timestamp, Raw: entry.Raw})
				pending[id] = pendingTool{Name: displayName, Kind: kind}
			case "function_call_output":
				id := stringValue(payload["call_id"])
				out := cleanCodexOutput(stringValue(payload["output"]))
				pt := pending[id]
				blocks = append(blocks, resultBlockFor(id, pt, out, entry.Timestamp, strings.Contains(stringValue(payload["output"]), "Process exited with code") && !strings.Contains(stringValue(payload["output"]), "code 0"), entry.Raw))
			case "custom_tool_call":
				id := stringValue(payload["call_id"])
				name := stringValue(payload["name"])
				inputText := stringValue(payload["input"])
				kind, displayName, input := codexCustomToolShape(name, inputText)
				blocks = append(blocks, Block{Kind: kind, Role: "assistant", ToolID: id, ToolName: displayName, ToolInput: input, Timestamp: entry.Timestamp, Raw: entry.Raw})
				pending[id] = pendingTool{Name: displayName, Kind: kind}
			case "custom_tool_call_output":
				id := stringValue(payload["call_id"])
				out, exitCode := customToolOutput(payload["output"])
				pt := pending[id]
				blocks = append(blocks, resultBlockFor(id, pt, out, entry.Timestamp, exitCode != 0, entry.Raw))
			}
		}
	}
	return blocks
}

func readJSONLines(data []byte) ([]jsonLine, error) {
	var entries []jsonLine
	s := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 0, 1024*1024)
	s.Buffer(buf, 32*1024*1024)
	for s.Scan() {
		line := bytes.TrimSpace(s.Bytes())
		if len(line) == 0 {
			continue
		}
		var entry jsonLine
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		entry.Raw = append(json.RawMessage(nil), line...)
		entries = append(entries, entry)
	}
	if err := s.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func decodeContentArray(raw json.RawMessage) []contentBlock {
	var arr []contentBlock
	_ = json.Unmarshal(raw, &arr)
	return arr
}

func isJSONString(raw json.RawMessage) bool {
	var s string
	return json.Unmarshal(raw, &s) == nil
}

func readJSONString(raw json.RawMessage) string {
	var s string
	_ = json.Unmarshal(raw, &s)
	return s
}

func toolResultText(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []contentBlock
	if json.Unmarshal(raw, &parts) == nil {
		texts := make([]string, 0, len(parts))
		for _, p := range parts {
			if p.Type == "text" && p.Text != "" {
				texts = append(texts, p.Text)
			}
		}
		return strings.Join(texts, "\n")
	}
	return string(raw)
}

func toolCallKind(name string) Kind {
	n := strings.ToLower(name)
	if n == "bash" || n == "shell" || n == "exec" || n == "exec_command" || n == "command_execution" {
		return KindCommand
	}
	return KindToolCall
}

func resultBlockFor(id string, pt pendingTool, text, ts string, isErr bool, raw json.RawMessage) Block {
	kind := KindToolResult
	if pt.Kind == KindCommand {
		kind = KindCommandResult
	}
	return Block{Kind: kind, Role: "tool", ToolID: id, ToolName: pt.Name, Text: strings.TrimSpace(text), Timestamp: ts, IsError: isErr, Raw: raw}
}

func codexCallShape(name, args string) (Kind, string, map[string]any) {
	input := map[string]any{}
	if args != "" {
		if err := json.Unmarshal([]byte(args), &input); err != nil {
			input = map[string]any{"raw": args}
		}
	}
	if name == "exec_command" {
		cmd := stringValue(input["cmd"])
		if workdir := stringValue(input["workdir"]); workdir != "" && cmd != "" {
			cmd = "cd " + workdir + " && " + cmd
		}
		return KindCommand, "Bash", map[string]any{"command": cmd}
	}
	if name == "apply_patch" {
		kind, display, parsed := codexCustomToolShape(name, args)
		return kind, display, parsed
	}
	return toolCallKind(name), name, input
}

func codexCustomToolShape(name, inputText string) (Kind, string, map[string]any) {
	if name == "apply_patch" {
		parsed := parseCodexPatch(inputText)
		display := "Edit"
		if v, _ := parsed["is_new"].(bool); v {
			display = "Write"
		}
		return KindToolCall, display, parsed
	}
	return toolCallKind(name), name, map[string]any{"raw": inputText}
}

func parseCodexPatch(patch string) map[string]any {
	lines := strings.Split(patch, "\n")
	filePath := ""
	isNew := false
	oldLines := []string{}
	newLines := []string{}
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "*** Begin Patch"), strings.HasPrefix(line, "*** End Patch"):
			continue
		case strings.HasPrefix(line, "*** Add File:"):
			filePath = strings.TrimSpace(strings.TrimPrefix(line, "*** Add File:"))
			isNew = true
			continue
		case strings.HasPrefix(line, "*** Update File:"):
			filePath = strings.TrimSpace(strings.TrimPrefix(line, "*** Update File:"))
			isNew = false
			continue
		case strings.HasPrefix(line, "@@"):
			continue
		case strings.HasPrefix(line, "+"):
			newLines = append(newLines, strings.TrimPrefix(line, "+"))
		case strings.HasPrefix(line, "-"):
			oldLines = append(oldLines, strings.TrimPrefix(line, "-"))
		default:
			oldLines = append(oldLines, line)
			newLines = append(newLines, line)
		}
	}
	out := map[string]any{"file_path": filePath, "is_new": isNew}
	if isNew {
		out["content"] = strings.Join(newLines, "\n")
	} else {
		out["old_string"] = strings.Join(oldLines, "\n")
		out["new_string"] = strings.Join(newLines, "\n")
	}
	return out
}

func customToolOutput(v any) (string, int) {
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x), 0
	case map[string]any:
		out := stringValue(x["output"])
		exitCode := 0
		if md, ok := x["metadata"].(map[string]any); ok {
			exitCode = int(numberValue(md["exit_code"]))
		}
		return strings.TrimSpace(out), exitCode
	default:
		b, _ := json.Marshal(v)
		return strings.TrimSpace(string(b)), 0
	}
}

func textFromCodexContent(v any, input bool) string {
	arr, ok := v.([]any)
	if !ok {
		return ""
	}
	var parts []string
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t := stringValue(m["type"])
		if input && t == "input_text" {
			parts = append(parts, stringValue(m["text"]))
		} else if !input && t == "output_text" {
			parts = append(parts, stringValue(m["text"]))
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func reasoningText(m map[string]any) string {
	if text := strings.TrimSpace(stringValue(m["text"])); text != "" {
		return text
	}
	if content := strings.TrimSpace(stringValue(m["content"])); content != "" {
		return content
	}
	if summary, ok := m["summary"].([]any); ok && len(summary) > 0 {
		var parts []string
		for _, item := range summary {
			if s := strings.TrimSpace(stringValue(item)); s != "" {
				parts = append(parts, s)
			} else if mm, ok := item.(map[string]any); ok {
				if s := strings.TrimSpace(stringValue(mm["text"])); s != "" {
					parts = append(parts, s)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	return ""
}

func extractCodexUserText(text string) string {
	const marker = "## My request for Codex:"
	if idx := strings.Index(text, marker); idx >= 0 {
		return strings.TrimSpace(text[idx+len(marker):])
	}
	const marker2 = "## My request for Codex"
	if idx := strings.Index(text, marker2); idx >= 0 {
		after := text[idx+len(marker2):]
		return strings.TrimSpace(strings.TrimLeft(after, ": \t\n"))
	}
	return strings.TrimSpace(text)
}

func cleanShellCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	cmd = strings.TrimPrefix(cmd, "/bin/bash -lc ")
	cmd = strings.Trim(cmd, "'\"")
	return cmd
}

func cleanCodexOutput(output string) string {
	lines := strings.Split(output, "\n")
	var kept []string
	for _, line := range lines {
		if strings.HasPrefix(line, "Chunk ID:") || strings.HasPrefix(line, "Wall time:") || strings.HasPrefix(line, "Process exited with code") || strings.HasPrefix(line, "Original token count:") || line == "Output:" {
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func cleanSystemTags(text string) string {
	replacements := []struct{ re, repl string }{
		{`(?s)<task-notification>\s*<task-id>[^<]*</task-id>\s*<output-file>[^<]*</output-file>\s*<status>([^<]*)</status>\s*<summary>([^<]*)</summary>\s*</task-notification>`, `[bg-task: $2]`},
		{`(?m)\n*Read the output file to retrieve the result:[^\n]*`, ``},
		{`(?s)<user_query>(.*?)</user_query>\s*`, `$1`},
		{`(?s)<system-reminder>.*?</system-reminder>\s*`, ``},
		{`(?s)<ide_opened_file>.*?</ide_opened_file>\s*`, ``},
		{`(?s)<local-command-caveat>.*?</local-command-caveat>\s*`, ``},
		{`(?s)<command-name>(.*?)</command-name>\s*`, `$1\n`},
		{`(?s)<command-message>.*?</command-message>\s*`, ``},
		{`(?s)<command-args>\s*</command-args>\s*`, ``},
		{`(?s)<local-command-stdout>.*?</local-command-stdout>\s*`, ``},
	}
	for _, r := range replacements {
		text = regexp.MustCompile(r.re).ReplaceAllString(text, r.repl)
	}
	text = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>\s*`).ReplaceAllStringFunc(text, func(s string) string {
		matches := regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`).FindStringSubmatch(s)
		if len(matches) < 2 || strings.TrimSpace(matches[1]) == "" {
			return ""
		}
		return strings.TrimSpace(matches[1]) + "\n"
	})
	return strings.TrimSpace(text)
}

func stringValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case fmt.Stringer:
		return x.String()
	case nil:
		return ""
	default:
		b, _ := json.Marshal(x)
		if string(b) == "null" {
			return ""
		}
		return string(b)
	}
}

func numberValue(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case json.Number:
		f, _ := x.Float64()
		return f
	default:
		return 0
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
