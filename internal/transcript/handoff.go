package transcript

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

type HandoffOptions struct {
	LastBlocks    int
	UserTurns     int
	MaxLines      int
	MaxFiles      int
	MaxCommands   int
	IncludeRecent bool
}

type HandoffRequest struct {
	Index int    `json:"index"`
	Turn  int    `json:"turn,omitempty"`
	Text  string `json:"text"`
}

type HandoffReport struct {
	Session            Session          `json:"session"`
	TotalBlocks        int              `json:"total_blocks"`
	TotalTurns         int              `json:"total_turns"`
	AnchorIndex        int              `json:"anchor_index"`
	RecentFrom         int              `json:"recent_from,omitempty"`
	RecentTo           int              `json:"recent_to,omitempty"`
	RecentTruncated    bool             `json:"recent_truncated,omitempty"`
	UserRequests       []HandoffRequest `json:"user_requests,omitempty"`
	RecentBlocks       []Block          `json:"recent_blocks,omitempty"`
	ChangedFiles       []FileRef        `json:"changed_files,omitempty"`
	ChangesPrecedeTask bool             `json:"changes_precede_task,omitempty"`
	RecentCommands     []CommandEntry   `json:"recent_commands,omitempty"`
	Activity           GitActivity      `json:"activity"`
	RenderOptions      RenderOptions    `json:"-"`
}

func BuildHandoff(path string, opts HandoffOptions) (HandoffReport, error) {
	tr, err := ParseFile(path)
	if err != nil {
		return HandoffReport{}, err
	}
	info, err := os.Stat(tr.Path)
	if err != nil {
		return HandoffReport{}, err
	}
	session := Session{
		Path: tr.Path, Provider: tr.Provider, ModTime: info.ModTime(), Size: info.Size(), Title: titleFromPath(tr.Path),
	}
	session = HydrateSessionFromTranscript(session, tr)

	if opts.LastBlocks < 0 {
		opts.LastBlocks = 0
	}
	if opts.UserTurns <= 0 {
		opts.UserTurns = 3
	}
	if opts.MaxLines < 0 {
		opts.MaxLines = 30
	}
	if opts.MaxFiles < 0 {
		opts.MaxFiles = 20
	}
	if opts.MaxCommands < 0 {
		opts.MaxCommands = 10
	}

	report := HandoffReport{Session: session, TotalBlocks: len(tr.Blocks), AnchorIndex: -1}
	for _, block := range tr.Blocks {
		if block.Turn > report.TotalTurns {
			report.TotalTurns = block.Turn
		}
	}

	continuationSeen := false
	substantiveCount := 0
	for i := len(tr.Blocks) - 1; i >= 0; i-- {
		block := tr.Blocks[i]
		if block.Kind != KindUser {
			continue
		}
		text := cleanPrompt(block.Text)
		if text == "" {
			continue
		}
		report.UserRequests = append(report.UserRequests, HandoffRequest{Index: block.Index, Turn: block.Turn, Text: text})
		if isContinuationRequest(text) {
			continuationSeen = true
		} else {
			substantiveCount++
		}
		neededSubstantive := 1
		if continuationSeen {
			neededSubstantive = 2
		}
		if len(report.UserRequests) >= opts.UserTurns && substantiveCount >= neededSubstantive {
			break
		}
		// Avoid an unbounded request header in pathological transcripts while still
		// reaching past a run of terse "continue" turns.
		if len(report.UserRequests) >= opts.UserTurns+4 {
			break
		}
	}
	for i, j := 0, len(report.UserRequests)-1; i < j; i, j = i+1, j-1 {
		report.UserRequests[i], report.UserRequests[j] = report.UserRequests[j], report.UserRequests[i]
	}
	for i := len(report.UserRequests) - 1; i >= 0; i-- {
		if !isContinuationRequest(report.UserRequests[i].Text) {
			report.AnchorIndex = report.UserRequests[i].Index
			break
		}
	}
	if report.AnchorIndex < 0 && len(tr.Blocks) > 0 {
		report.AnchorIndex = tr.Blocks[len(tr.Blocks)-1].Index
	}
	taskTranscript := tr
	if report.AnchorIndex >= 0 {
		taskTranscript.Blocks = blocksFromIndex(tr.Blocks, report.AnchorIndex)
	}
	report.Activity = GitAndPRActivity(taskTranscript)

	renderOpts := RenderOptions{
		HideThinking: true, HideTools: true, HideToolResults: true, ShowTurns: true, MaxLines: opts.MaxLines,
	}
	report.RenderOptions = renderOpts
	if opts.IncludeRecent && len(tr.Blocks) > 0 {
		visible := FilterBlocks(taskTranscript.Blocks, renderOpts)
		start := 0
		if opts.LastBlocks > 0 && len(visible) > opts.LastBlocks {
			start = len(visible) - opts.LastBlocks
			report.RecentTruncated = true
		}
		report.RecentBlocks = limitRecentCommands(visible[start:], opts.MaxCommands)
		if len(report.RecentBlocks) > 0 {
			report.RecentFrom = report.RecentBlocks[0].Index
			report.RecentTo = report.RecentBlocks[len(report.RecentBlocks)-1].Index
		}
	}

	files := FileRefs(taskTranscript, true)
	if len(files) == 0 && report.AnchorIndex > 0 {
		files = FileRefs(tr, true)
		report.ChangesPrecedeTask = len(files) > 0
	}
	sort.SliceStable(files, func(i, j int) bool {
		return lastIndex(files[i].Indexes) > lastIndex(files[j].Indexes)
	})
	if opts.MaxFiles > 0 && len(files) > opts.MaxFiles {
		files = files[:opts.MaxFiles]
	}
	if report.ChangesPrecedeTask && len(files) > 10 {
		files = files[:10]
	}
	report.ChangedFiles = files

	commands, err := CommandEntries(taskTranscript, false, "", false, false, false)
	if err != nil {
		return HandoffReport{}, err
	}
	if opts.MaxCommands > 0 && len(commands) > opts.MaxCommands {
		commands = commands[len(commands)-opts.MaxCommands:]
	}
	report.RecentCommands = commands
	return report, nil
}

func RenderHandoff(w io.Writer, report HandoffReport, format string) error {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "md", "markdown", "text", "plain":
		renderHandoffMarkdown(w, report)
		return nil
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	default:
		return fmt.Errorf("handoff supports --format md or json, got %q", format)
	}
}

func renderHandoffMarkdown(w io.Writer, report HandoffReport) {
	path := report.Session.Path
	_, _ = fmt.Fprintln(w, "# Agent continuation context")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "> Continue from this transcript, but treat the current filesystem/repository state as authoritative. Verify live state before changing code or repeating completed work.")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "- **Source:** `%s`\n", path)
	_, _ = fmt.Fprintf(w, "- **Provider:** `%s`\n", report.Session.Provider)
	if report.Session.Project != "" {
		_, _ = fmt.Fprintf(w, "- **Project:** `%s`\n", report.Session.Project)
	}
	if report.Session.CWD != "" {
		_, _ = fmt.Fprintf(w, "- **Working directory:** `%s`\n", report.Session.CWD)
	}
	if report.Session.ID != "" {
		_, _ = fmt.Fprintf(w, "- **Session:** `%s`\n", report.Session.ID)
	}
	_, _ = fmt.Fprintf(w, "- **Transcript:** %d blocks, %d user turns\n", report.TotalBlocks, report.TotalTurns)
	if len(report.RecentBlocks) > 0 {
		_, _ = fmt.Fprintf(w, "- **Recent window:** `#%03d–#%03d`", report.RecentFrom, report.RecentTo)
		if report.RecentTruncated {
			_, _ = fmt.Fprint(w, " (earlier blocks omitted from this handoff)")
		}
		_, _ = fmt.Fprintln(w)
	}

	if len(report.UserRequests) > 0 {
		_, _ = fmt.Fprintln(w, "\n## Latest user requests")
		for _, request := range report.UserRequests {
			_, _ = fmt.Fprintf(w, "\n### Turn %d · #%03d\n\n%s\n", request.Turn, request.Index, request.Text)
		}
	}

	if len(report.RecentBlocks) > 0 {
		_, _ = fmt.Fprintln(w, "\n## Recent work trace")
		_, _ = fmt.Fprintln(w, "\nThinking and bulky tool/command results are hidden here. Stable block indexes are preserved for drill-down.")
		_, _ = fmt.Fprintln(w)
		for _, block := range report.RecentBlocks {
			if block.Kind == KindUser {
				continue
			}
			_, _ = fmt.Fprintf(w, "### %s\n\n", markdownHeader(block, report.RenderOptions))
			body, collapsed := renderedBody(block, path, report.RenderOptions)
			if body == "" {
				body = toolInputText(block)
			}
			if block.Kind == KindCommand {
				_, _ = fmt.Fprintf(w, "`%s`\n", markdownInline(truncateHandoff(commandSummaryText(block), 320)))
				collapsed = false
			} else if block.Kind == KindCommandResult || block.Kind == KindToolCall || block.Kind == KindToolResult {
				_, _ = fmt.Fprintf(w, "```text\n%s\n```\n", strings.TrimRight(body, "\n"))
			} else {
				_, _ = fmt.Fprintf(w, "%s\n", body)
			}
			if collapsed {
				_, _ = fmt.Fprintf(w, "\n_Expand block:_ `agentscript open %q --around %d --expand %d`\n", path, block.Index, block.Index)
			}
			_, _ = fmt.Fprintln(w)
		}
	}

	if len(report.ChangedFiles) > 0 {
		heading := "## Files changed during the current task"
		if report.ChangesPrecedeTask {
			heading = "## Earlier files changed in this session"
		}
		_, _ = fmt.Fprintln(w, heading)
		_, _ = fmt.Fprintln(w)
		if report.ChangesPrecedeTask {
			_, _ = fmt.Fprintf(w, "No file edits were detected after the current task anchor `#%03d`; these earlier edits may still be relevant.\n\n", report.AnchorIndex)
		}
		for _, ref := range report.ChangedFiles {
			idx := lastIndex(ref.Indexes)
			op := strings.Join(ref.Ops, ", ")
			if op == "" {
				op = "change"
			}
			_, _ = fmt.Fprintf(w, "- `%s` — %s, last seen at `#%03d`\n", ref.Path, op, idx)
		}
		_, _ = fmt.Fprintln(w)
	}

	renderHandoffActivity(w, report.Activity)

	_, _ = fmt.Fprintln(w, "## Drill down")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintf(w, "- More continuation context: `agentscript handoff %q --last 240`\n", path)
	if report.AnchorIndex >= 0 {
		_, _ = fmt.Fprintf(w, "- Inspect around the current task anchor: `agentscript open %q --around %d --before 80 --after 120 --profile handoff`\n", path, report.AnchorIndex)
	}
	_, _ = fmt.Fprintf(w, "- All changed files: `agentscript changes %q`\n", path)
	_, _ = fmt.Fprintf(w, "- Git/validation/failure activity: `agentscript activity %q`\n", path)
	_, _ = fmt.Fprintf(w, "- Full normalized transcript: `agentscript open %q`\n", path)
}

func renderHandoffActivity(w io.Writer, activity GitActivity) {
	type section struct {
		name  string
		items []string
	}
	sections := []section{
		{"Branches", activity.Branches}, {"Pull requests", activity.PRs}, {"Pushes", activity.Pushes},
		{"Validations", activity.Validations}, {"Failures", activity.Failures},
	}
	any := false
	for _, section := range sections {
		if len(section.items) > 0 {
			any = true
			break
		}
	}
	if !any {
		return
	}
	_, _ = fmt.Fprintln(w, "## Git, validation, and failures")
	_, _ = fmt.Fprintln(w)
	for _, section := range sections {
		if len(section.items) == 0 {
			continue
		}
		_, _ = fmt.Fprintf(w, "**%s**\n\n", section.name)
		items := section.items
		if len(items) > 8 {
			items = items[len(items)-8:]
		}
		for _, item := range items {
			_, _ = fmt.Fprintf(w, "- %s\n", truncateHandoff(item, 260))
		}
		_, _ = fmt.Fprintln(w)
	}
}

func lastIndex(indexes []int) int {
	if len(indexes) == 0 {
		return 0
	}
	return indexes[len(indexes)-1]
}

func truncateHandoff(value string, max int) string {
	value = strings.Join(strings.Fields(value), " ")
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max-1] + "…"
}

func markdownInline(value string) string {
	return strings.ReplaceAll(value, "`", "'")
}

func isContinuationRequest(value string) bool {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.Trim(normalized, ".!… ")
	switch normalized {
	case "continue", "keep going", "go on", "proceed", "resume", "continue please", "keep going please":
		return true
	default:
		return false
	}
}

func limitRecentCommands(blocks []Block, maxCommands int) []Block {
	if maxCommands <= 0 {
		return blocks
	}
	keepCommand := map[int]bool{}
	count := 0
	for i := len(blocks) - 1; i >= 0 && count < maxCommands; i-- {
		if blocks[i].Kind == KindCommand {
			keepCommand[blocks[i].Index] = true
			count++
		}
	}
	out := make([]Block, 0, len(blocks))
	for _, block := range blocks {
		if block.Kind == KindCommand && !keepCommand[block.Index] {
			continue
		}
		out = append(out, block)
	}
	return out
}

func blocksFromIndex(blocks []Block, from int) []Block {
	for i, block := range blocks {
		if block.Index >= from {
			return blocks[i:]
		}
	}
	return nil
}
