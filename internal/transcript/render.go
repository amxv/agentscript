package transcript

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

func Render(w io.Writer, tr Transcript, opts RenderOptions) error {
	blocks := FilterBlocks(tr.Blocks, opts)
	switch opts.normalizedFormat() {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Path     string   `json:"path,omitempty"`
			Provider Provider `json:"provider"`
			Blocks   []Block  `json:"blocks"`
		}{Path: tr.Path, Provider: tr.Provider, Blocks: blocks})
	case "md", "markdown":
		renderMarkdown(w, tr, blocks, opts)
		return nil
	case "text", "plain", "":
		renderText(w, tr, blocks, opts)
		return nil
	default:
		return fmt.Errorf("unsupported format %q", opts.Format)
	}
}

func FilterBlocks(blocks []Block, opts RenderOptions) []Block {
	out := make([]Block, 0, len(blocks))
	for _, b := range blocks {
		if opts.MessagesOnly && b.Kind != KindUser && b.Kind != KindAssistant {
			continue
		}
		if opts.HideThinking && b.Kind == KindThinking {
			continue
		}
		if opts.HideTools && (b.Kind == KindToolCall || b.Kind == KindToolResult) {
			continue
		}
		if opts.HideToolResults && (b.Kind == KindToolResult || b.Kind == KindCommandResult) {
			continue
		}
		if opts.HideCommands && (b.Kind == KindCommand || b.Kind == KindCommandResult) {
			continue
		}
		if len(opts.OnlyTools) > 0 && isToolish(b.Kind) && !nameInList(b.ToolName, opts.OnlyTools) {
			continue
		}
		if len(opts.HideToolNames) > 0 && isToolish(b.Kind) && nameInList(b.ToolName, opts.HideToolNames) {
			continue
		}
		out = append(out, b)
	}
	return out
}

func renderText(w io.Writer, tr Transcript, blocks []Block, opts RenderOptions) {
	if tr.Path != "" {
		_, _ = fmt.Fprintf(w, "agentscript transcript: %s (%s)\n\n", tr.Path, tr.Provider)
	}
	for _, b := range blocks {
		header := blockHeader(b, opts)
		_, _ = fmt.Fprintln(w, header)
		body := blockBodyText(b)
		if body != "" {
			_, _ = fmt.Fprintln(w, indentRight(strings.TrimRight(body, "\n"), "  "))
		}
		_, _ = fmt.Fprintln(w)
	}
}

func renderMarkdown(w io.Writer, tr Transcript, blocks []Block, opts RenderOptions) {
	_, _ = fmt.Fprintf(w, "# agentscript transcript")
	if tr.Path != "" {
		_, _ = fmt.Fprintf(w, ": `%s`", tr.Path)
	}
	_, _ = fmt.Fprintf(w, "\n\nProvider: `%s`\n\n", tr.Provider)
	for _, b := range blocks {
		_, _ = fmt.Fprintf(w, "## %s\n\n", markdownHeader(b, opts))
		body := blockBodyText(b)
		if body == "" {
			body = toolInputText(b)
		}
		if b.Kind == KindCommand || b.Kind == KindCommandResult || b.Kind == KindToolCall || b.Kind == KindToolResult {
			_, _ = fmt.Fprintf(w, "```text\n%s\n```\n\n", strings.TrimRight(body, "\n"))
		} else {
			_, _ = fmt.Fprintf(w, "%s\n\n", body)
		}
	}
}

func blockHeader(b Block, opts RenderOptions) string {
	label := string(b.Kind)
	switch b.Kind {
	case KindUser:
		label = "user"
	case KindAssistant:
		label = "assistant"
	case KindThinking:
		label = "thinking"
	case KindCommand:
		label = "command"
	case KindCommandResult:
		label = "command_result"
	case KindToolCall:
		label = "tool_call"
	case KindToolResult:
		label = "tool_result"
	}
	parts := []string{fmt.Sprintf("#%03d", b.Index), label}
	if b.ToolName != "" {
		parts = append(parts, b.ToolName)
	}
	if b.IsError {
		parts = append(parts, "ERROR")
	}
	if opts.ShowTimestamps && b.Timestamp != "" {
		parts = append(parts, b.Timestamp)
	}
	return strings.Join(parts, " ")
}

func markdownHeader(b Block, opts RenderOptions) string {
	return strings.TrimPrefix(blockHeader(b, opts), "#")
}

func blockBodyText(b Block) string {
	switch b.Kind {
	case KindToolCall, KindCommand:
		return toolInputText(b)
	default:
		return b.Text
	}
}

func toolInputText(b Block) string {
	if b.Kind == KindCommand {
		if cmd, ok := b.ToolInput["command"]; ok {
			return fmt.Sprint(cmd)
		}
	}
	if len(b.ToolInput) == 0 {
		return ""
	}
	keys := sortedKeys(b.ToolInput)
	var lines []string
	for _, k := range keys {
		v := b.ToolInput[k]
		switch x := v.(type) {
		case string:
			lines = append(lines, fmt.Sprintf("%s: %s", k, x))
		default:
			buf, _ := json.MarshalIndent(x, "", "  ")
			lines = append(lines, fmt.Sprintf("%s: %s", k, string(buf)))
		}
	}
	return strings.Join(lines, "\n")
}

func indentRight(s, prefix string) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	for i := range lines {
		lines[i] = prefix + lines[i]
	}
	return strings.Join(lines, "\n")
}

func isToolish(kind Kind) bool {
	switch kind {
	case KindToolCall, KindToolResult, KindCommand, KindCommandResult:
		return true
	default:
		return false
	}
}

func nameInList(name string, names []string) bool {
	for _, n := range names {
		if strings.EqualFold(strings.TrimSpace(n), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func FormatSummaryBlock(b Block) string {
	text := strings.ReplaceAll(blockBodyText(b), "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	if len(text) > 120 {
		text = text[:117] + "..."
	}
	if b.ToolName != "" {
		return fmt.Sprintf("#%03d %-15s %-12s %s", b.Index, b.Kind, b.ToolName, text)
	}
	return fmt.Sprintf("#%03d %-15s %s", b.Index, b.Kind, text)
}

func commaList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if v := strings.TrimSpace(p); v != "" {
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
