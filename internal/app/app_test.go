package app

import (
	"bytes"
	"os"
	"path/filepath"
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

func TestRootHelpIncludesCodexSessionCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"--help"}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run help: %v", err)
	}
	if !strings.Contains(stdout.String(), "codex <session-id>") {
		t.Fatalf("root help missing Codex session command:\n%s", stdout.String())
	}
}

func TestCodexSessionCommand(t *testing.T) {
	const sessionID = "019f91bc-123f-7692-8a78-21e54d6677e6"
	root := t.TempDir()
	sessionDir := filepath.Join(root, "2026", "07", "24")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sessionDir, "rollout-2026-07-24T06-57-29-"+sessionID+".jsonl")
	fixture := `{"type":"session_meta","payload":{"id":"` + sessionID + `"}}` + "\n" +
		`{"type":"event_msg","payload":{"type":"user_message","message":"opened by session ID"}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	err := Run([]string{"codex", sessionID, "--roots", root}, os.Stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run codex session ID: %v", err)
	}
	if !strings.Contains(stdout.String(), "opened by session ID") {
		t.Fatalf("output missing transcript message:\n%s", stdout.String())
	}
}

func TestCodexHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"codex", "--help"}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run codex help: %v", err)
	}
	for _, want := range []string{"codex <session-id>", "$CODEX_HOME/sessions", "--roots"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("codex help missing %q:\n%s", want, stdout.String())
		}
	}
}
