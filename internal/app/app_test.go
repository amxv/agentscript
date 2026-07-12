package app

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"--help"}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run help: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"agentscript", "open [path]", "slice <path> <range>", "search <query>"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q in:\n%s", want, out)
		}
	}
}

func TestVersion(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"--version"}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run version: %v", err)
	}
	if !strings.HasPrefix(stdout.String(), "agentscript ") {
		t.Fatalf("unexpected version output: %q", stdout.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run([]string{"wat"}, os.Stdin, &stdout, &stderr)
	if err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("expected unknown command error, got %v", err)
	}
}

func TestOpenHelpIncludesInternalGoalFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"open", "help"}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run open help: %v", err)
	}
	for _, flag := range []string{"--show-internal-goal", "--show-internal"} {
		if !strings.Contains(stdout.String(), flag) {
			t.Fatalf("open help missing %s:\n%s", flag, stdout.String())
		}
	}
}
