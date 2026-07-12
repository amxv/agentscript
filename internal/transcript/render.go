package transcript

import (
	"encoding/json"
	"fmt"
	"html"
	"io"
	"regexp"
	"sort"
	"strings"
)

var internalGoalPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<codex_internal_goal\b[^>]*>.*?</codex_internal_goal\s*>\s*`),
	regexp.MustCompile(`(?is)<codex_internal_context\b[^>]*\bsource\s*=\s*["']goal["'][^>]*>.*?</codex_internal_context\s*>\s*`),
}

var internalContextPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?is)<environment_context\b[^>]*>.*?</environment_context\s*>\s*`),
	regexp.MustCompile(`(?is)<system-reminder\b[^>]*>.*?</system-reminder\s*>\s*`),
	regexp.MustCompile(`(?is)<ide_opened_file\b[^>]*>.*?</ide_opened_file\s*>\s*`),
	regexp.MustCompile(`(?is)<local-command-caveat\b[^>]*>.*?</local-command-caveat\s*>\s*`),
}

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
	case "html":
		renderHTML(w, tr, blocks, opts)
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
		if len(opts.OnlyKinds) > 0 && !kindInList(b.Kind, opts.OnlyKinds) {
			continue
		}
		if len(opts.HideKinds) > 0 && kindInList(b.Kind, opts.HideKinds) {
			continue
		}
		if len(opts.OnlyTools) > 0 && isToolish(b.Kind) && !nameInList(b.ToolName, opts.OnlyTools) {
			continue
		}
		if len(opts.HideToolNames) > 0 && isToolish(b.Kind) && nameInList(b.ToolName, opts.HideToolNames) {
			continue
		}
		if !opts.ShowInternal && isInjectedAgentsInstructions(b.Text) {
			continue
		}
		if !opts.ShowInternal && b.Text != "" {
			b.Text = stripInternalContextBlocks(b.Text)
		}
		if !opts.ShowInternal && !opts.ShowInternalGoal && b.Text != "" {
			b.Text = stripInternalGoalBlocks(b.Text)
		}
		out = append(out, b)
	}
	return out
}

func isInjectedAgentsInstructions(text string) bool {
	trimmed := strings.TrimSpace(text)
	return strings.HasPrefix(trimmed, "# AGENTS.md instructions for ") && strings.Contains(trimmed, "<INSTRUCTIONS>")
}

func stripInternalContextBlocks(text string) string {
	for _, pattern := range internalContextPatterns {
		text = pattern.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}

func stripInternalGoalBlocks(text string) string {
	for _, pattern := range internalGoalPatterns {
		text = pattern.ReplaceAllString(text, "")
	}
	return strings.TrimSpace(text)
}

func renderText(w io.Writer, tr Transcript, blocks []Block, opts RenderOptions) {
	if tr.Path != "" {
		_, _ = fmt.Fprintf(w, "agentscript transcript: %s (%s)\n\n", tr.Path, tr.Provider)
	}
	for _, b := range blocks {
		header := blockHeader(b, opts)
		_, _ = fmt.Fprintln(w, header)
		body, collapsed := renderedBody(b, tr.Path, opts)
		if body != "" {
			_, _ = fmt.Fprintln(w, indentRight(strings.TrimRight(body, "\n"), "  "))
		}
		if collapsed {
			_, _ = fmt.Fprintf(w, "  expand: agentscript open %q --around %d --expand %d\n", tr.Path, b.Index, b.Index)
		}
		_, _ = fmt.Fprintln(w)
	}
}

func renderMarkdown(w io.Writer, tr Transcript, blocks []Block, opts RenderOptions) {
	style := strings.ToLower(strings.TrimSpace(opts.MarkdownStyle))
	if style == "llm" {
		style = "llm-context"
	}
	if style == "llm-context" {
		_, _ = fmt.Fprintf(w, "# Transcript context\n\n")
		_, _ = fmt.Fprintf(w, "Source: `%s`\nProvider: `%s`\n\n", tr.Path, tr.Provider)
	} else if style == "audit" {
		_, _ = fmt.Fprintf(w, "# agentscript audit export\n\n- Source: `%s`\n- Provider: `%s`\n- Blocks: `%d`\n\n", tr.Path, tr.Provider, len(blocks))
	} else {
		_, _ = fmt.Fprintf(w, "# agentscript transcript")
		if tr.Path != "" {
			_, _ = fmt.Fprintf(w, ": `%s`", tr.Path)
		}
		_, _ = fmt.Fprintf(w, "\n\nProvider: `%s`\n\n", tr.Provider)
	}
	for _, b := range blocks {
		if style == "compact" {
			_, _ = fmt.Fprintf(w, "### %s\n\n", markdownHeader(b, opts))
		} else {
			_, _ = fmt.Fprintf(w, "## %s\n\n", markdownHeader(b, opts))
		}
		body, collapsed := renderedBody(b, tr.Path, opts)
		if body == "" {
			body = toolInputText(b)
		}
		if b.Kind == KindCommand || b.Kind == KindCommandResult || b.Kind == KindToolCall || b.Kind == KindToolResult {
			_, _ = fmt.Fprintf(w, "```text\n%s\n```\n", strings.TrimRight(body, "\n"))
		} else {
			_, _ = fmt.Fprintf(w, "%s\n", body)
		}
		if collapsed {
			_, _ = fmt.Fprintf(w, "\n_expand with:_ `agentscript open %q --around %d --expand %d`\n", tr.Path, b.Index, b.Index)
		}
		_, _ = fmt.Fprintln(w)
	}
}

func renderHTML(w io.Writer, tr Transcript, blocks []Block, opts RenderOptions) {
	_, _ = fmt.Fprintf(w, "<!doctype html><html><head><meta charset=\"utf-8\"><meta name=\"viewport\" content=\"width=device-width,initial-scale=1\"><title>agentscript transcript</title><style>%s</style></head><body>", htmlCSS())
	_, _ = fmt.Fprintf(w, "<header><h1>agentscript transcript</h1><p><code>%s</code> · <strong>%s</strong> · %d blocks</p><input id=\"q\" placeholder=\"Search visible blocks…\" oninput=\"filterBlocks()\"></header><main>", html.EscapeString(tr.Path), html.EscapeString(string(tr.Provider)), len(blocks))
	for _, b := range blocks {
		body, collapsed := renderedBody(b, tr.Path, opts)
		classes := "block kind-" + strings.ReplaceAll(string(b.Kind), "_", "-")
		_, _ = fmt.Fprintf(w, "<section class=\"%s\" data-search=\"%s\">", classes, html.EscapeString(strings.ToLower(blockHeader(b, opts)+" "+body)))
		_, _ = fmt.Fprintf(w, "<div class=\"block-head\"><a id=\"b%d\" href=\"#b%d\">#%03d</a><span>%s</span>", b.Index, b.Index, b.Index, html.EscapeString(string(b.Kind)))
		if b.ToolName != "" {
			_, _ = fmt.Fprintf(w, "<span>%s</span>", html.EscapeString(b.ToolName))
		}
		if opts.ShowTurns && b.Turn > 0 {
			_, _ = fmt.Fprintf(w, "<span>turn:%d</span>", b.Turn)
		}
		if b.IsError {
			_, _ = fmt.Fprintf(w, "<span class=\"err\">ERROR</span>")
		}
		_, _ = fmt.Fprint(w, "</div>")
		if collapsed {
			_, _ = fmt.Fprintf(w, "<details><summary>Collapsed to %d lines. Expand block.</summary><pre>%s</pre></details>", opts.MaxLines, html.EscapeString(blockBodyText(b)))
		} else {
			_, _ = fmt.Fprintf(w, "<pre>%s</pre>", html.EscapeString(body))
		}
		_, _ = fmt.Fprint(w, "</section>")
	}
	_, _ = fmt.Fprintf(w, "</main><script>%s</script></body></html>", htmlJS())
}

func renderedBody(b Block, path string, opts RenderOptions) (string, bool) {
	body := blockBodyText(b)
	if body == "" {
		body = toolInputText(b)
	}
	if opts.MaxLines <= 0 || opts.ExpandAll || intInList(b.Index, opts.ExpandIndexes) {
		return body, false
	}
	lines := strings.Split(body, "\n")
	if len(lines) <= opts.MaxLines {
		return body, false
	}
	kept := append([]string{}, lines[:opts.MaxLines]...)
	kept = append(kept, fmt.Sprintf("… collapsed %d more lines from block #%03d", len(lines)-opts.MaxLines, b.Index))
	return strings.Join(kept, "\n"), true
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
	if opts.ShowTurns && b.Turn > 0 {
		parts = append(parts, fmt.Sprintf("turn:%d", b.Turn))
	}
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

func kindInList(kind Kind, kinds []Kind) bool {
	for _, k := range kinds {
		if k == kind {
			return true
		}
	}
	return false
}

func nameInList(name string, names []string) bool {
	for _, n := range names {
		if strings.EqualFold(strings.TrimSpace(n), strings.TrimSpace(name)) {
			return true
		}
	}
	return false
}

func intInList(v int, vals []int) bool {
	for _, x := range vals {
		if x == v {
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

func htmlCSS() string {
	return `:root{font-family:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;color:#111;background:#fafafa}body{margin:0}header{position:sticky;top:0;background:rgba(250,250,250,.94);backdrop-filter:blur(10px);border-bottom:1px solid #ddd;padding:18px 24px;z-index:2}h1{font-size:20px;margin:0 0 6px}p{margin:0 0 12px;color:#555}input{width:min(720px,100%);padding:10px 12px;border:1px solid #ccc;border-radius:10px;font:inherit}main{max-width:1100px;margin:0 auto;padding:24px}.block{background:#fff;border:1px solid #e4e4e4;border-radius:14px;margin:0 0 14px;overflow:hidden;box-shadow:0 1px 2px rgba(0,0,0,.03)}.block-head{display:flex;gap:10px;align-items:center;background:#f4f4f4;border-bottom:1px solid #e8e8e8;padding:9px 12px;font-size:13px}.block-head a{font-weight:700;color:#111;text-decoration:none}.block-head span{background:#fff;border:1px solid #e0e0e0;border-radius:999px;padding:2px 8px}.block-head .err{background:#fee;color:#a00;border-color:#fbb}pre{white-space:pre-wrap;word-break:break-word;margin:0;padding:14px;font:13px/1.5 ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace}details summary{cursor:pointer;padding:14px;color:#555}.kind-user{border-left:4px solid #111}.kind-assistant{border-left:4px solid #666}.kind-thinking{opacity:.82}.kind-command,.kind-command-result{border-left:4px solid #999}.kind-tool-call,.kind-tool-result{border-left:4px solid #bbb}`
}

func htmlJS() string {
	return `function filterBlocks(){const q=document.getElementById('q').value.toLowerCase();for(const el of document.querySelectorAll('.block')){el.style.display=el.dataset.search.includes(q)?'block':'none'}}`
}
