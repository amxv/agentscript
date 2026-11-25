package transcript

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

type agyStep struct {
	Index     int    `json:"step_index"`
	Type      string `json:"type"`
	Source    string `json:"source"`
	Status    string `json:"status"`
	Timestamp string `json:"created_at"`
	Content   string `json:"content"`
	Thinking  string `json:"thinking"`
	Error     string `json:"error"`
	ExitCode  *int   `json:"exit_code"`
	Calls     []struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	} `json:"tool_calls"`
	Media []struct {
		MIME string `json:"mime_type"`
	} `json:"media"`
	raw json.RawMessage
}

func isAntigravityRecord(obj map[string]any) bool {
	kind, _ := obj["type"].(string)
	source, _ := obj["source"].(string)
	return int64Pointer(obj["step_index"]) != nil && kind != "" && kind == strings.ToUpper(kind) && source != ""
}

func agyUserText(text string) string {
	if rest, ok := strings.CutPrefix(strings.TrimSpace(text), "<USER_REQUEST>"); ok {
		if end := strings.LastIndex(rest, "</USER_REQUEST>"); end >= 0 {
			return strings.TrimSpace(rest[:end])
		}
	}
	return text
}

func agyToolCall(name, id string, args map[string]any) Block {
	b := Block{Kind: KindToolCall, Role: "assistant", ToolName: name, ToolID: id, ToolInput: args}
	if name == "run_command" {
		b.Kind = KindCommand
		if cmd, ok := args["CommandLine"].(string); ok {
			args["command"] = cmd
		}
	}
	return b
}

var agyCommandExit = regexp.MustCompile(`(?i)^(?:the )?command exited with code (-?[0-9]+)\.`)

// Native tool results can have two layers of generated timing headers.
func agyResultText(text string) string {
	for {
		line, rest, found := strings.Cut(text, "\n")
		if !found || !(strings.HasPrefix(line, "Created At: ") || strings.HasPrefix(line, "Completed At: ")) {
			return strings.TrimSpace(text)
		}
		text = rest
	}
}

func agyToolFailed(status, detail string, exitCode *int, kind Kind, output string) bool {
	if exitCode != nil && *exitCode != 0 {
		return true
	}
	if detail != "" || status == "ERROR" || status == "CANCELED" || status == "INTERRUPTED" || status == "failed" {
		return true
	}
	if kind != KindCommand {
		return false
	}
	match := agyCommandExit.FindStringSubmatch(output)
	return len(match) > 0 && match[1] != "0"
}

func parseAntigravity(data []byte) ([]Block, error) {
	var steps []agyStep
	err := scanJSONLines(data, func(entry jsonLine) {
		var step agyStep
		if json.Unmarshal(entry.Raw, &step) == nil && step.Source != "" {
			step.raw = append(json.RawMessage(nil), entry.Raw...)
			steps = append(steps, step)
		}
	})
	if err != nil {
		return nil, err
	}
	// agy may flush parallel tool steps in completion order.
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].Index < steps[j].Index })
	var blocks []Block
	pending := map[int]Block{}
	for _, s := range steps {
		content := s.Content
		if s.Type == "USER_INPUT" {
			content = agyUserText(content)
		}
		for _, media := range s.Media {
			marker := "[Media attachment]"
			if strings.HasPrefix(media.MIME, "image/") {
				marker = imageAttachment
			}
			content = strings.TrimSpace(content + "\n" + marker)
		}
		add := func(b Block) {
			b.Timestamp = s.Timestamp
			b.Raw = s.raw
			blocks = append(blocks, b)
		}
		switch s.Type {
		case "USER_INPUT":
			add(Block{Kind: KindUser, Role: "user", Text: content})
		case "PLANNER_RESPONSE":
			add(Block{Kind: KindThinking, Role: "assistant", Text: s.Thinking})
			add(Block{Kind: KindAssistant, Role: "assistant", Text: content})
			for i, c := range s.Calls {
				call := agyToolCall(c.Name, fmt.Sprintf("step-%d-call-%d", s.Index, i), c.Args)
				pending[s.Index+1+i] = call
				add(call)
			}
		default:
			call, paired := pending[s.Index]
			if (s.Source == "SYSTEM" || s.Type == "CHECKPOINT") && !(paired && s.Type == "ERROR_MESSAGE") {
				if content == "" {
					content = s.Error
				}
				add(Block{Kind: KindSystem, Role: "system", Text: content, IsError: s.Error != "" || s.Type == "ERROR_MESSAGE"})
				continue
			}
			if !paired {
				call = Block{ToolName: strings.ToLower(s.Type), Kind: KindToolCall}
				if s.Type == "RUN_COMMAND" {
					call.Kind = KindCommand
				}
			}
			content = agyResultText(content)
			if s.Error != "" && !strings.Contains(content, s.Error) {
				content = strings.TrimSpace(content + "\n" + s.Error)
			}
			failed := s.Type == "ERROR_MESSAGE" || agyToolFailed(s.Status, s.Error, s.ExitCode, call.Kind, content)
			add(resultBlockFor(call.ToolID, pendingTool{Name: call.ToolName, Kind: call.Kind}, content, s.Timestamp, failed, s.raw))
		}
	}
	return blocks, nil
}
