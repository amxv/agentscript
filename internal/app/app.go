package app

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/amxv/agentscript/internal/buildinfo"
	"github.com/amxv/agentscript/internal/transcript"
)

const commandName = "agentscript"

var version = buildinfo.CurrentVersion()

func Run(args []string, stdin *os.File, stdout, stderr io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printRootHelp(stdout)
		return nil
	}
	if len(args) == 1 && isVersionArg(args[0]) {
		_, _ = fmt.Fprintf(stdout, "%s %s\n", commandName, version)
		return nil
	}

	switch args[0] {
	case "open":
		return runOpen(args[1:], stdin, stdout, stderr)
	case "slice":
		return runSlice(args[1:], stdout)
	case "list", "ls":
		return runList(args[1:], stdout)
	case "search":
		return runSearch(args[1:], stdout)
	case "commands", "cmds":
		return runCommands(args[1:], stdout)
	case "files":
		return runFiles(args[1:], stdout, false)
	case "changes":
		return runFiles(args[1:], stdout, true)
	case "activity", "git", "pr":
		return runActivity(args[1:], stdout)
	case "export":
		return runExport(args[1:], stdout)
	case "split":
		return runSplit(args[1:], stdout)
	case "config":
		return runConfig(args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q (run `%s --help`)", args[0], commandName)
	}
}

type commonFlags struct {
	format           string
	out              string
	profile          string
	kinds            string
	hideKinds        string
	maxLines         int
	expand           string
	markdownStyle    string
	hideThinking     bool
	hideTools        bool
	hideToolResults  bool
	hideCommands     bool
	messagesOnly     bool
	showTimestamps   bool
	showTurns        bool
	showInternalGoal bool
	showInternal     bool
	showThinking     bool
	showTools        bool
	showToolResults  bool
	showCommands     bool
	tools            string
	hideToolNames    string
}

func (c commonFlags) renderOptions() (transcript.RenderOptions, error) {
	opts := transcript.RenderOptions{}
	cfg, err := transcript.LoadConfig()
	if err != nil {
		return opts, err
	}
	opts, err = transcript.ApplyProfile(opts, cfg, c.profile)
	if err != nil {
		return opts, err
	}
	if c.format != "" {
		opts.Format = c.format
	}
	if c.hideThinking {
		opts.HideThinking = true
	}
	if c.showThinking {
		opts.HideThinking = false
	}
	if c.hideTools {
		opts.HideTools = true
	}
	if c.showTools {
		opts.HideTools = false
	}
	if c.hideToolResults {
		opts.HideToolResults = true
	}
	if c.showToolResults {
		opts.HideToolResults = false
	}
	if c.hideCommands {
		opts.HideCommands = true
	}
	if c.showCommands {
		opts.HideCommands = false
	}
	if c.messagesOnly {
		opts.MessagesOnly = true
	}
	if c.showTimestamps {
		opts.ShowTimestamps = true
	}
	if c.showTurns {
		opts.ShowTurns = true
	}
	if c.showInternalGoal {
		opts.ShowInternalGoal = true
	}
	if c.showInternal {
		opts.ShowInternal = true
		opts.ShowInternalGoal = true
	}
	if c.tools != "" {
		opts.OnlyTools = splitCSV(c.tools)
	}
	if c.hideToolNames != "" {
		opts.HideToolNames = splitCSV(c.hideToolNames)
	}
	if c.kinds != "" {
		opts.OnlyKinds = transcript.ParseKinds(splitCSV(c.kinds))
	}
	if c.hideKinds != "" {
		opts.HideKinds = transcript.ParseKinds(splitCSV(c.hideKinds))
	}
	if c.maxLines >= 0 {
		opts.MaxLines = c.maxLines
	}
	if c.expand != "" {
		if strings.EqualFold(c.expand, "all") {
			opts.ExpandAll = true
		} else {
			idxs, err := parseIndexList(c.expand)
			if err != nil {
				return opts, err
			}
			opts.ExpandIndexes = idxs
		}
	}
	if c.markdownStyle != "" {
		opts.MarkdownStyle = c.markdownStyle
	}
	return opts, nil
}

func addCommonFlags(fs *flag.FlagSet, c *commonFlags) {
	fs.StringVar(&c.format, "format", "", "output format: text, md, html, json")
	fs.StringVar(&c.out, "out", "", "write output to file instead of stdout")
	fs.StringVar(&c.profile, "profile", "", "render profile: full, compact, messages, handoff, debug, commands, tools, or config-defined")
	fs.StringVar(&c.kinds, "kind", "", "only show comma-separated block kinds: user,assistant,thinking,tool_call,tool_result,command,command_result")
	fs.StringVar(&c.hideKinds, "hide-kind", "", "hide comma-separated block kinds")
	fs.BoolVar(&c.hideThinking, "hide-thinking", false, "hide thinking blocks")
	fs.BoolVar(&c.showThinking, "show-thinking", false, "show thinking blocks, overriding profile/config")
	fs.BoolVar(&c.hideThinking, "no-thinking", false, "alias for --hide-thinking")
	fs.BoolVar(&c.hideTools, "hide-tools", false, "hide non-command tool calls and results")
	fs.BoolVar(&c.showTools, "show-tools", false, "show non-command tool calls/results, overriding profile/config")
	fs.BoolVar(&c.hideTools, "no-tools", false, "alias for --hide-tools")
	fs.BoolVar(&c.hideToolResults, "hide-tool-results", false, "hide tool and command result blocks")
	fs.BoolVar(&c.showToolResults, "show-tool-results", false, "show tool and command result blocks, overriding profile/config")
	fs.BoolVar(&c.hideToolResults, "no-results", false, "alias for --hide-tool-results")
	fs.BoolVar(&c.hideCommands, "hide-commands", false, "hide shell command blocks and their results")
	fs.BoolVar(&c.showCommands, "show-commands", false, "show command blocks/results, overriding profile/config")
	fs.BoolVar(&c.messagesOnly, "messages-only", false, "show only user and assistant messages")
	fs.BoolVar(&c.showTimestamps, "timestamps", false, "show timestamps in block headers")
	fs.BoolVar(&c.showTurns, "turns", false, "show user-turn numbers in block headers")
	fs.BoolVar(&c.showInternalGoal, "show-internal-goal", false, "show Codex internal goal/context blocks")
	fs.BoolVar(&c.showInternal, "show-internal", false, "show injected runtime instructions and context")
	fs.StringVar(&c.tools, "tools", "", "only show tool/command blocks with these comma-separated names")
	fs.StringVar(&c.hideToolNames, "hide-tool", "", "hide tool/command blocks with these comma-separated names")
	fs.IntVar(&c.maxLines, "max-lines", -1, "collapse block bodies longer than N lines; use 0 to disable")
	fs.StringVar(&c.expand, "expand", "", "expand collapsed block index list like 12,18 or all")
	fs.StringVar(&c.markdownStyle, "md-style", "", "markdown style: compact, llm-context, audit")
}

func runOpen(args []string, stdin *os.File, stdout, stderr io.Writer) error {
	var c commonFlags
	var pathFlag string
	var sliceSpec string
	var turnSlice string
	var latest int
	var provider string
	var roots string
	var noTUI bool
	from, to, last, around, before, after := -1, -1, 0, -1, 25, 50

	fs := flag.NewFlagSet("open", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addCommonFlags(fs, &c)
	fs.StringVar(&pathFlag, "path", "", "path to a Claude Code or Codex JSONL transcript")
	fs.StringVar(&pathFlag, "p", "", "alias for --path")
	fs.StringVar(&sliceSpec, "slice", "", "slice by stable block index, e.g. 0:100, 100:, :50")
	fs.StringVar(&turnSlice, "turn-slice", "", "slice by user-turn number, e.g. 1:5")
	fs.IntVar(&from, "from", -1, "slice start block index")
	fs.IntVar(&to, "to", -1, "slice end block index, inclusive")
	fs.IntVar(&last, "last", 0, "show the last N renderable blocks")
	fs.IntVar(&around, "around", -1, "show blocks around this index")
	fs.IntVar(&before, "before", 25, "number of blocks before --around")
	fs.IntVar(&after, "after", 50, "number of blocks after --around")
	fs.IntVar(&latest, "latest", 0, "open the Nth latest transcript; 1 is most recent")
	fs.StringVar(&provider, "provider", "", "provider filter for latest/picker: claude or codex")
	fs.StringVar(&roots, "roots", "", "comma-separated roots for discovery; defaults to ~/.claude/projects,~/.codex/sessions")
	fs.BoolVar(&noTUI, "no-tui", false, "do not launch the picker when no path is provided")
	if err := fs.Parse(interspersed(args, openValueFlags())); err != nil {
		return err
	}
	if fs.NArg() > 0 && isHelpArg(fs.Arg(0)) {
		printOpenHelp(stdout)
		return nil
	}
	path := pathFlag
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	if path == "" {
		sessions, err := transcript.Discover(80, parseProvider(provider), splitCSV(roots))
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			return errors.New("no transcripts found in default roots; pass --path or add --roots")
		}
		if latest > 0 {
			if latest > len(sessions) {
				return fmt.Errorf("--latest %d requested, but only %d transcripts were found", latest, len(sessions))
			}
			path = sessions[latest-1].Path
		} else if noTUI {
			printSessions(stdout, sessions, 20)
			return nil
		} else {
			picked, err := pickSession(stdin, stdout, sessions)
			if err != nil {
				printSessions(stdout, sessions, 20)
				_, _ = fmt.Fprintln(stderr, "Picker unavailable; pass one of the paths above with `agentscript open <path>`.")
				return nil
			}
			path = picked.Path
		}
	}
	return renderPath(path, stdout, c, sliceSpec, turnSlice, from, to, last, around, before, after)
}

func runSlice(args []string, stdout io.Writer) error {
	var c commonFlags
	fs := flag.NewFlagSet("slice", flag.ContinueOnError)
	addCommonFlags(fs, &c)
	if err := fs.Parse(interspersed(args, commonValueFlags())); err != nil {
		return err
	}
	if fs.NArg() < 2 || isHelpArg(fs.Arg(0)) {
		printSliceHelp(stdout)
		return nil
	}
	return renderPath(fs.Arg(0), stdout, c, fs.Arg(1), "", -1, -1, 0, -1, 25, 50)
}

func runList(args []string, stdout io.Writer) error {
	var latest int
	var provider, roots string
	fs := flag.NewFlagSet("list", flag.ContinueOnError)
	fs.IntVar(&latest, "latest", 50, "number of latest transcripts to show")
	fs.StringVar(&provider, "provider", "", "provider filter: claude or codex")
	fs.StringVar(&roots, "roots", "", "comma-separated roots for discovery")
	if err := fs.Parse(interspersed(args, listValueFlags())); err != nil {
		return err
	}
	if fs.NArg() > 0 && isHelpArg(fs.Arg(0)) {
		printListHelp(stdout)
		return nil
	}
	sessions, err := transcript.Discover(latest, parseProvider(provider), splitCSV(roots))
	if err != nil {
		return err
	}
	printSessions(stdout, sessions, latest)
	return nil
}

func runSearch(args []string, stdout io.Writer) error {
	var c commonFlags
	var latest, near int
	var provider, roots, kind, tool string
	var regex, caseSensitive, all bool
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	addCommonFlags(fs, &c)
	fs.IntVar(&latest, "latest", 100, "number of latest transcripts to search")
	fs.StringVar(&provider, "provider", "", "provider filter: claude or codex")
	fs.StringVar(&roots, "roots", "", "comma-separated roots for discovery")
	fs.StringVar(&kind, "search-kind", "", "only search comma-separated block kinds")
	fs.StringVar(&tool, "tool", "", "only search comma-separated tool/command names")
	fs.BoolVar(&regex, "regex", false, "treat queries as regular expressions")
	fs.BoolVar(&caseSensitive, "case-sensitive", false, "case-sensitive search")
	fs.BoolVar(&all, "all", false, "require all query args to match")
	fs.IntVar(&near, "near", 0, "require all query args to appear within N blocks")
	if err := fs.Parse(interspersed(args, searchValueFlags())); err != nil {
		return err
	}
	if fs.NArg() == 0 || isHelpArg(fs.Arg(0)) {
		printSearchHelp(stdout)
		return nil
	}
	queries := fs.Args()
	sessions, err := transcript.Discover(latest, parseProvider(provider), splitCSV(roots))
	if err != nil {
		return err
	}
	renderOpts, err := c.renderOptions()
	if err != nil {
		return err
	}
	mode := transcript.SearchAny
	if all || near > 0 {
		mode = transcript.SearchAll
	}
	searchKinds := transcript.ParseKinds(splitCSV(kind))
	if kind == "" && len(renderOpts.OnlyKinds) > 0 {
		searchKinds = renderOpts.OnlyKinds
	}
	searchTools := splitCSV(tool)
	if tool == "" && len(renderOpts.OnlyTools) > 0 {
		searchTools = renderOpts.OnlyTools
	}
	matches, err := transcript.SearchAdvanced(sessions, transcript.SearchOptions{
		Queries: queries, Mode: mode, Near: near, Regex: regex, CaseSensitive: caseSensitive,
		Kinds: searchKinds, Tools: searchTools, RenderOptions: renderOpts,
	})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(stdout, "%d matches across %d transcripts\n\n", len(matches), len(sessions))
	for i, m := range matches {
		_, _ = fmt.Fprintf(stdout, "[%d] %s  %s  %s\n", i+1, m.Session.Provider, formatTime(m.Session.ModTime), m.Session.Title)
		_, _ = fmt.Fprintf(stdout, "    %s\n", transcript.FormatSummaryBlock(m.Block))
		if m.Snippet != "" {
			_, _ = fmt.Fprintf(stdout, "    %s\n", m.Snippet)
		}
		_, _ = fmt.Fprintf(stdout, "    open: %s open %q --around %d\n\n", commandName, m.Session.Path, m.Block.Index)
	}
	return nil
}

func renderPath(path string, stdout io.Writer, c commonFlags, sliceSpec, turnSlice string, from, to, last, around, before, after int) error {
	tr, err := transcript.ParseFile(path)
	if err != nil {
		return err
	}
	if sliceSpec == "" && (from >= 0 || to >= 0) {
		left, right := "", ""
		if from >= 0 {
			left = strconv.Itoa(from)
		}
		if to >= 0 {
			right = strconv.Itoa(to)
		}
		sliceSpec = left + ":" + right
	}
	if turnSlice != "" {
		spec, err := transcript.ParseSliceSpec(turnSlice)
		if err != nil {
			return err
		}
		tr.Blocks = transcript.SliceBlocksByTurn(tr.Blocks, spec)
	}
	if sliceSpec != "" || last > 0 || around >= 0 {
		spec, err := transcript.ParseSliceSpec(sliceSpec)
		if err != nil {
			return err
		}
		tr.Blocks = transcript.SliceBlocks(tr.Blocks, spec, last, around, before, after)
	}
	writer := stdout
	var file *os.File
	if c.out != "" {
		if err := os.MkdirAll(filepath.Dir(c.out), 0o755); err != nil && filepath.Dir(c.out) != "." {
			return err
		}
		f, err := os.Create(c.out)
		if err != nil {
			return err
		}
		defer f.Close()
		file = f
		writer = file
	}
	opts, err := c.renderOptions()
	if err != nil {
		return err
	}
	return transcript.Render(writer, tr, opts)
}

func pickSession(stdin *os.File, stdout io.Writer, sessions []transcript.Session) (transcript.Session, error) {
	if stdin == nil {
		return transcript.Session{}, errors.New("stdin unavailable")
	}
	if len(sessions) == 0 {
		return transcript.Session{}, errors.New("no sessions")
	}
	if !isTerminal(stdin) {
		return transcript.Session{}, errors.New("stdin is not a terminal")
	}
	restore, _ := enableRawMode(stdin)
	if restore != nil {
		defer restore()
	}
	reader := bufio.NewReader(stdin)
	limit := len(sessions)
	if limit > 30 {
		limit = 30
	}
	selected := 0
	for {
		_, _ = fmt.Fprint(stdout, "\033[H\033[2J")
		_, _ = fmt.Fprintln(stdout, "agentscript open — latest transcripts")
		_, _ = fmt.Fprintln(stdout, "↑/↓ or j/k to move, enter to open, q to quit")
		_, _ = fmt.Fprintln(stdout)
		for i := 0; i < limit; i++ {
			prefix := "  "
			if i == selected {
				prefix = "> "
			}
			s := sessions[i]
			_, _ = fmt.Fprintf(stdout, "%s%2d. %-6s %-16s %s\n", prefix, i+1, s.Provider, formatTime(s.ModTime), s.Title)
		}
		b, err := reader.ReadByte()
		if err != nil {
			return transcript.Session{}, err
		}
		switch b {
		case 'q', 3, 27:
			// Escape may be an arrow prefix; try to read the rest without blocking too badly.
			if b == 27 {
				b2, _ := reader.ReadByte()
				b3, _ := reader.ReadByte()
				if b2 == '[' {
					switch b3 {
					case 'A':
						if selected > 0 {
							selected--
						}
					case 'B':
						if selected < limit-1 {
							selected++
						}
					}
					continue
				}
			}
			return transcript.Session{}, errors.New("cancelled")
		case 'k':
			if selected > 0 {
				selected--
			}
		case 'j':
			if selected < limit-1 {
				selected++
			}
		case '\r', '\n':
			return sessions[selected], nil
		}
	}
}

func enableRawMode(stdin *os.File) (func(), error) {
	get := exec.Command("stty", "-g")
	get.Stdin = stdin
	state, err := get.Output()
	if err != nil {
		return nil, err
	}
	raw := exec.Command("stty", "raw", "-echo")
	raw.Stdin = stdin
	if err := raw.Run(); err != nil {
		return nil, err
	}
	return func() {
		restore := exec.Command("stty", strings.TrimSpace(string(state)))
		restore.Stdin = stdin
		_ = restore.Run()
	}, nil
}

func isTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func printSessions(w io.Writer, sessions []transcript.Session, limit int) {
	if limit <= 0 || limit > len(sessions) {
		limit = len(sessions)
	}
	_, _ = fmt.Fprintln(w, "Provider  Time              Title / Path")
	for i := 0; i < limit; i++ {
		s := sessions[i]
		_, _ = fmt.Fprintf(w, "%2d. %-7s %-16s %s\n", i+1, s.Provider, formatTime(s.ModTime), s.Title)
		_, _ = fmt.Fprintf(w, "    %s\n", s.Path)
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04")
}

func commonValueFlags() map[string]bool {
	return map[string]bool{
		"format": true, "out": true, "tools": true, "hide-tool": true, "profile": true, "kind": true, "hide-kind": true, "max-lines": true, "expand": true, "md-style": true,
	}
}

func openValueFlags() map[string]bool {
	m := commonValueFlags()
	for _, k := range []string{"path", "p", "slice", "turn-slice", "from", "to", "last", "around", "before", "after", "latest", "provider", "roots"} {
		m[k] = true
	}
	return m
}

func listValueFlags() map[string]bool {
	return map[string]bool{"latest": true, "provider": true, "roots": true}
}

func searchValueFlags() map[string]bool {
	m := commonValueFlags()
	for _, k := range []string{"latest", "provider", "roots", "search-kind", "tool", "near"} {
		m[k] = true
	}
	return m
}

func interspersed(args []string, valueFlags map[string]bool) []string {
	flags := []string{}
	positional := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if eq := strings.Index(name, "="); eq >= 0 {
			name = name[:eq]
		}
		if valueFlags[name] && !strings.Contains(arg, "=") && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func parseProvider(v string) transcript.Provider {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "claude", "claude-code":
		return transcript.ProviderClaude
	case "codex":
		return transcript.ProviderCodex
	default:
		return ""
	}
}

func splitCSV(s string) []string {
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
	return out
}

func isVersionArg(v string) bool { return v == "--version" }

func isHelpArg(v string) bool {
	switch v {
	case "-h", "--help", "help":
		return true
	default:
		return false
	}
}

func printRootHelp(w io.Writer) {
	writeLines(w,
		"agentscript - readable Claude Code and Codex transcript CLI",
		"",
		"Usage:",
		"  agentscript [--version]",
		"  agentscript <command> [arguments]",
		"",
		"Commands:",
		"  open [path]       open/render a transcript, or pick from latest sessions",
		"  slice <path> <range>  render a stable block-index slice like 0:100",
		"  search <query>    search latest Claude/Codex transcripts",
		"  list              list latest discovered transcripts",
		"  commands          show shell commands and optional outputs",
		"  files             show files referenced in a transcript",
		"  changes           show files changed in a transcript",
		"  activity          summarize Git/PR/push/test activity",
		"  export            export a transcript to text, Markdown, JSON, or HTML",
		"  split             split a transcript into multiple rendered files",
		"  config            show or initialize agentscript config",
		"",
		"Examples:",
		"  agentscript open ~/.claude/projects/.../session.jsonl",
		"  agentscript open --path ~/.codex/sessions/.../rollout.jsonl --hide-thinking",
		"  agentscript open transcript.jsonl --slice 0:100 --out context.md --format md",
		"  agentscript slice transcript.jsonl 100: --messages-only",
		"  agentscript search \"publish-pr\" --provider codex",
		"  agentscript list --latest 20",
	)
}

func printOpenHelp(w io.Writer) {
	writeLines(w,
		"agentscript open - open or render a transcript",
		"",
		"Usage:",
		"  agentscript open [path] [flags]",
		"  agentscript open --path <path> [flags]",
		"  agentscript open --latest 1 [flags]",
		"",
		"Important flags:",
		"  --slice 0:100          render block indexes 0 through 100",
		"  --from 0 --to 100      same as --slice 0:100",
		"  --last 80              render the last 80 blocks",
		"  --around 100           render around block #100",
		"  --hide-thinking        hide thinking blocks",
		"  --show-internal-goal   show Codex internal goal/context blocks",
		"  --show-internal        show all injected runtime instructions/context",
		"  --hide-tools           hide non-command tool calls/results",
		"  --hide-commands        hide shell commands/results",
		"  --hide-tool Bash       hide a named tool/command",
		"  --messages-only        show only user/assistant messages",
		"  --format text|md|json  output format",
		"  --out file             write output to file",
	)
}

func printSliceHelp(w io.Writer) {
	writeLines(w,
		"agentscript slice - render a stable block-index range",
		"",
		"Usage:",
		"  agentscript slice <path> <range> [flags]",
		"",
		"Examples:",
		"  agentscript slice transcript.jsonl 0:100",
		"  agentscript slice transcript.jsonl 100: --out new-task.md --format md",
	)
}

func printSearchHelp(w io.Writer) {
	writeLines(w,
		"agentscript search - search latest transcripts",
		"",
		"Usage:",
		"  agentscript search <query> [flags]",
		"",
		"Examples:",
		"  agentscript search \"r2 cors\"",
		"  agentscript search publish-pr --provider codex --latest 20",
	)
}

func printListHelp(w io.Writer) {
	writeLines(w,
		"agentscript list - list latest discovered transcripts",
		"",
		"Usage:",
		"  agentscript list [--latest 50] [--provider claude|codex]",
	)
}

func writeLines(w io.Writer, lines ...string) {
	for _, line := range lines {
		_, _ = fmt.Fprintln(w, line)
	}
}
