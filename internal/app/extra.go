package app

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/amxv/agentscript/internal/transcript"
)

func runCommands(args []string, stdout io.Writer) error {
	var format, grep string
	var failed, withOutput, regex, caseSensitive bool
	fs := flag.NewFlagSet("commands", flag.ContinueOnError)
	fs.StringVar(&format, "format", "text", "output format: text or json")
	fs.BoolVar(&failed, "failed", false, "only show failed commands")
	fs.BoolVar(&withOutput, "with-output", false, "include command output")
	fs.StringVar(&grep, "grep", "", "filter commands/results by text")
	fs.BoolVar(&regex, "regex", false, "treat --grep as a regular expression")
	fs.BoolVar(&caseSensitive, "case-sensitive", false, "case-sensitive grep/regex")
	if err := fs.Parse(interspersed(args, map[string]bool{"format": true, "grep": true})); err != nil {
		return err
	}
	if fs.NArg() < 1 || isHelpArg(fs.Arg(0)) {
		printCommandsHelp(stdout)
		return nil
	}
	tr, err := transcript.ParseFile(fs.Arg(0))
	if err != nil {
		return err
	}
	entries, err := transcript.CommandEntries(tr, failed, grep, regex, caseSensitive, withOutput)
	if err != nil {
		return err
	}
	return transcript.RenderCommands(stdout, entries, format)
}

func runFiles(args []string, stdout io.Writer, changedOnly bool) error {
	var format string
	fs := flag.NewFlagSet("files", flag.ContinueOnError)
	fs.StringVar(&format, "format", "text", "output format: text or json")
	if err := fs.Parse(interspersed(args, map[string]bool{"format": true})); err != nil {
		return err
	}
	if fs.NArg() < 1 || isHelpArg(fs.Arg(0)) {
		printFilesHelp(stdout, changedOnly)
		return nil
	}
	tr, err := transcript.ParseFile(fs.Arg(0))
	if err != nil {
		return err
	}
	refs := transcript.FileRefs(tr, changedOnly)
	return transcript.RenderFileRefs(stdout, refs, format)
}

func runActivity(args []string, stdout io.Writer) error {
	var format string
	fs := flag.NewFlagSet("activity", flag.ContinueOnError)
	fs.StringVar(&format, "format", "text", "output format: text or json")
	if err := fs.Parse(interspersed(args, map[string]bool{"format": true})); err != nil {
		return err
	}
	if fs.NArg() < 1 || isHelpArg(fs.Arg(0)) {
		printActivityHelp(stdout)
		return nil
	}
	tr, err := transcript.ParseFile(fs.Arg(0))
	if err != nil {
		return err
	}
	activity := transcript.GitAndPRActivity(tr)
	return transcript.RenderGitActivity(stdout, activity, format)
}

func runExport(args []string, stdout io.Writer) error {
	var c commonFlags
	var sliceSpec, turnSlice string
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	addCommonFlags(fs, &c)
	fs.StringVar(&sliceSpec, "slice", "", "slice by stable block index")
	fs.StringVar(&turnSlice, "turn-slice", "", "slice by user-turn number")
	if err := fs.Parse(interspersed(args, exportValueFlags())); err != nil {
		return err
	}
	if fs.NArg() < 1 || isHelpArg(fs.Arg(0)) {
		printExportHelp(stdout)
		return nil
	}
	if c.out == "" {
		return errors.New("export requires --out")
	}
	return renderPath(fs.Arg(0), stdout, c, sliceSpec, turnSlice, -1, -1, 0, -1, 25, 50)
}

func runSplit(args []string, stdout io.Writer) error {
	var c commonFlags
	var at, every int
	var outDir, prefix string
	fs := flag.NewFlagSet("split", flag.ContinueOnError)
	addCommonFlags(fs, &c)
	fs.IntVar(&at, "at", -1, "split once after this block index")
	fs.IntVar(&every, "every", 0, "split into chunks of N blocks")
	fs.StringVar(&outDir, "out-dir", ".", "directory for split files")
	fs.StringVar(&prefix, "prefix", "transcript", "output filename prefix")
	if err := fs.Parse(interspersed(args, splitValueFlags())); err != nil {
		return err
	}
	if fs.NArg() < 1 || isHelpArg(fs.Arg(0)) {
		printSplitHelp(stdout)
		return nil
	}
	if at < 0 && every <= 0 {
		return errors.New("split requires --at or --every")
	}
	tr, err := transcript.ParseFile(fs.Arg(0))
	if err != nil {
		return err
	}
	opts, err := c.renderOptions()
	if err != nil {
		return err
	}
	format := opts.Format
	if format == "" {
		format = "md"
		opts.Format = format
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	var chunks [][]transcript.Block
	if at >= 0 {
		left := transcript.SliceBlocks(tr.Blocks, transcript.SliceSpec{From: ptrInt(0), To: ptrInt(at)}, 0, -1, 0, 0)
		right := transcript.SliceBlocks(tr.Blocks, transcript.SliceSpec{From: ptrInt(at + 1)}, 0, -1, 0, 0)
		chunks = append(chunks, left, right)
	} else {
		for i := 0; i < len(tr.Blocks); i += every {
			end := i + every
			if end > len(tr.Blocks) {
				end = len(tr.Blocks)
			}
			chunks = append(chunks, tr.Blocks[i:end])
		}
	}
	ext := extensionForFormat(format)
	for i, chunk := range chunks {
		if len(chunk) == 0 {
			continue
		}
		path := filepath.Join(outDir, fmt.Sprintf("%s.part-%02d.%s", prefix, i+1, ext))
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		part := tr
		part.Blocks = chunk
		if err := transcript.Render(f, part, opts); err != nil {
			_ = f.Close()
			return err
		}
		_ = f.Close()
		_, _ = fmt.Fprintf(stdout, "%s\n", path)
	}
	return nil
}

func runConfig(args []string, stdout io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) || args[0] == "show" {
		cfg, err := transcript.LoadConfig()
		if err != nil {
			return err
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Path   string            `json:"path"`
			Config transcript.Config `json:"config"`
		}{Path: transcript.ConfigPath(), Config: cfg})
	}
	switch args[0] {
	case "path":
		_, _ = fmt.Fprintln(stdout, transcript.ConfigPath())
		return nil
	case "init":
		path := transcript.ConfigPath()
		if len(args) > 1 {
			path = args[1]
		}
		if err := transcript.WriteDefaultConfig(path); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, "wrote %s\n", path)
		return nil
	default:
		return fmt.Errorf("unknown config command %q", args[0])
	}
}

func parseIndexList(s string) ([]int, error) {
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(strings.TrimPrefix(part, "#"))
		if part == "" {
			continue
		}
		v, err := strconv.Atoi(part)
		if err != nil || v < 0 {
			return nil, fmt.Errorf("invalid block index %q", part)
		}
		out = append(out, v)
	}
	return out, nil
}

func ptrInt(v int) *int { return &v }

func extensionForFormat(format string) string {
	switch strings.ToLower(format) {
	case "html":
		return "html"
	case "json":
		return "json"
	case "txt", "text", "plain":
		return "txt"
	default:
		return "md"
	}
}

func exportValueFlags() map[string]bool {
	m := commonValueFlags()
	m["slice"] = true
	m["turn-slice"] = true
	return m
}

func splitValueFlags() map[string]bool {
	m := commonValueFlags()
	m["at"] = true
	m["every"] = true
	m["out-dir"] = true
	m["prefix"] = true
	return m
}

func printCommandsHelp(w io.Writer) {
	writeLines(w,
		"agentscript commands - show shell commands from a transcript",
		"",
		"Usage:",
		"  agentscript commands <path> [--failed] [--with-output] [--grep text]",
	)
}

func printFilesHelp(w io.Writer, changedOnly bool) {
	name := "files"
	if changedOnly {
		name = "changes"
	}
	writeLines(w,
		"agentscript "+name+" - show files referenced or changed in a transcript",
		"",
		"Usage:",
		"  agentscript "+name+" <path> [--format text|json]",
	)
}

func printActivityHelp(w io.Writer) {
	writeLines(w,
		"agentscript activity - summarize Git, PR, push, validation, and failure activity",
		"",
		"Usage:",
		"  agentscript activity <path> [--format text|json]",
	)
}

func printExportHelp(w io.Writer) {
	writeLines(w,
		"agentscript export - export transcript output to a file",
		"",
		"Usage:",
		"  agentscript export <path> --format html --out transcript.html",
		"  agentscript export <path> --format md --md-style llm-context --slice 0:100 --out context.md",
	)
}

func printSplitHelp(w io.Writer) {
	writeLines(w,
		"agentscript split - split a transcript into rendered files",
		"",
		"Usage:",
		"  agentscript split <path> --at 100 --out-dir parts --format md",
		"  agentscript split <path> --every 80 --out-dir parts --format html",
	)
}
