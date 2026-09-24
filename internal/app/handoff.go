package app

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/amxv/agentscript/internal/transcript"
)

func runHandoff(args []string, stdout io.Writer) error {
	if len(args) == 0 || (len(args) == 1 && isHelpArg(args[0])) {
		printHandoffHelp(stdout)
		return nil
	}
	var format, out, roots string
	var last, userTurns, maxLines, maxFiles, maxCommands int
	fs := flag.NewFlagSet("handoff", flag.ContinueOnError)
	fs.StringVar(&format, "format", "md", "output format: md or json")
	fs.StringVar(&out, "out", "", "write output to a file instead of stdout")
	fs.StringVar(&roots, "roots", "", "comma-separated roots for session-ID lookup")
	fs.IntVar(&last, "last", 40, "recent visible handoff blocks to include; 0 includes all")
	fs.IntVar(&userTurns, "user-turns", 3, "latest meaningful user requests to repeat at the top")
	fs.IntVar(&maxLines, "max-lines", 30, "fold recent trace blocks longer than N lines; 0 disables folding")
	fs.IntVar(&maxFiles, "files", 20, "maximum changed files to summarize; 0 means all")
	fs.IntVar(&maxCommands, "commands", 10, "maximum recent commands to summarize; 0 means all")
	if err := fs.Parse(interspersed(args, map[string]bool{
		"format": true, "out": true, "roots": true, "last": true, "user-turns": true,
		"max-lines": true, "files": true, "commands": true,
	})); err != nil {
		return err
	}
	if fs.NArg() == 0 || isHelpArg(fs.Arg(0)) {
		printHandoffHelp(stdout)
		return nil
	}

	path, err := transcript.ResolveSessionReference(fs.Arg(0), splitCSV(roots))
	if err != nil {
		return err
	}
	report, err := transcript.BuildHandoff(path, transcript.HandoffOptions{
		LastBlocks: last, UserTurns: userTurns, MaxLines: maxLines, MaxFiles: maxFiles, MaxCommands: maxCommands, IncludeRecent: true,
	})
	if err != nil {
		return err
	}

	writer := stdout
	var file *os.File
	if out != "" {
		if dir := filepath.Dir(out); dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
		}
		file, err = os.Create(out)
		if err != nil {
			return err
		}
		defer file.Close()
		writer = file
	}
	return transcript.RenderHandoff(writer, report, strings.ToLower(strings.TrimSpace(format)))
}

func printHandoffHelp(w io.Writer) {
	writeLines(w,
		"agentscript handoff - continuation-ready context for another agent",
		"",
		"Use this first when a user gives you a transcript path and asks you to continue prior work.",
		"It repeats the latest user requests, recent execution trace, changed files, recent commands,",
		"Git/validation/failure activity, and exact commands for deeper inspection.",
		"",
		"Usage:",
		"  agentscript handoff <path-or-session-id> [flags]",
		"  agentscript continue <path-or-session-id> [flags]",
		"  agentscript <path.jsonl> [flags]        # bare-path shortcut",
		"",
		"Important flags:",
		"  --last 40             recent visible handoff blocks to include; 0 means all",
		"  --user-turns 3        repeat the latest meaningful user requests",
		"  --max-lines 30        fold long recent blocks",
		"  --format md|json      handoff output format",
		"  --out file            write the handoff to a file",
		"",
		"Examples:",
		"  agentscript handoff ~/.codex/sessions/.../rollout.jsonl",
		"  agentscript continue 019f91bc-123f-7692-8a78-21e54d6677e6",
		"  agentscript handoff transcript.jsonl --last 240",
	)
}

func handoffCommand(path string) string {
	return fmt.Sprintf("%s handoff %q", commandName, path)
}
