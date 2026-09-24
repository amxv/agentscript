package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/amxv/agentscript/internal/transcript"
)

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"--help"}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run help: %v", err)
	}
	out := stdout.String()
	for _, want := range []string{"agentscript", "open [path]", "slice <path> <range>", "search <query>", "compactions"} {
		if !strings.Contains(out, want) {
			t.Fatalf("help missing %q in:\n%s", want, out)
		}
	}
}

func TestCompactionsCommandJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude.jsonl")
	fixture := `{"type":"user","timestamp":"2026-08-03T02:00:00Z","message":{"role":"user","content":"hello"}}
{"type":"system","timestamp":"2026-08-03T02:01:00Z","subtype":"compact_boundary","compactMetadata":{"trigger":"manual","preTokens":100,"postTokens":20,"durationMs":5}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if err := Run([]string{"compactions", "--format", "json", path}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run compactions: %v", err)
	}
	if !strings.Contains(stdout.String(), `"provider": "claude"`) || !strings.Contains(stdout.String(), `"post_tokens": 20`) {
		t.Fatalf("unexpected compactions output:\n%s", stdout.String())
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

func TestSearchCommandUsesGlobalSearchSemanticsAndJSONGroups(t *testing.T) {
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	fixtures := map[string]string{
		"one.jsonl": "push rejected by remote",
		"two.jsonl": "push accepted by remote",
	}
	for name, message := range fixtures {
		path := filepath.Join(root, name)
		fixture := `{"type":"user","timestamp":"2026-09-20T10:00:00Z","cwd":"/work/agentscript","sessionId":"` + name + `","message":{"role":"user","content":"` + message + `"}}` + "\n"
		if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	var stdout, stderr bytes.Buffer
	err := Run([]string{"search", "push", "rejected", "--roots", root, "--provider", "claude", "--format", "json"}, os.Stdin, &stdout, &stderr)
	if err != nil {
		t.Fatalf("Run search: %v", err)
	}
	var result struct {
		Stats struct {
			MatchedFiles int `json:"matched_files"`
		} `json:"stats"`
		Groups []struct {
			Session transcript.Session `json:"session"`
			Count   int                `json:"count"`
		} `json:"groups"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatalf("decode search JSON: %v\n%s", err, stdout.String())
	}
	if result.Stats.MatchedFiles != 1 || len(result.Groups) != 1 || result.Groups[0].Count != 1 {
		t.Fatalf("unexpected search result: %+v", result)
	}
	if result.Groups[0].Session.Project != "agentscript" {
		t.Fatalf("project = %q, want agentscript", result.Groups[0].Session.Project)
	}
}

func TestIndexStatusCommand(t *testing.T) {
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	path := filepath.Join(root, "one.jsonl")
	fixture := `{"type":"user","timestamp":"2026-09-20T10:00:00Z","message":{"role":"user","content":"hello"}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"index", "status", "--roots", root, "--provider", "claude", "--format", "json"}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run index status: %v", err)
	}
	if !strings.Contains(stdout.String(), `"sessions": 1`) || !strings.Contains(stdout.String(), `"missing": 1`) {
		t.Fatalf("unexpected index status:\n%s", stdout.String())
	}
}

func TestBareTranscriptPathRunsHandoff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handoff.jsonl")
	fixture := `{"type":"user","timestamp":"2026-09-20T10:00:00Z","cwd":"/work/demo","sessionId":"session-1","message":{"role":"user","content":"package the release"}}` + "\n" +
		`{"type":"assistant","timestamp":"2026-09-20T10:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"I started the release work."}]}}` + "\n" +
		`{"type":"user","timestamp":"2026-09-20T10:00:02Z","cwd":"/work/demo","sessionId":"session-1","message":{"role":"user","content":"keep going"}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := Run([]string{path}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run bare handoff: %v", err)
	}
	for _, want := range []string{"# Agent continuation context", "package the release", "keep going", "## Drill down"} {
		if !strings.Contains(stdout.String(), want) {
			t.Fatalf("handoff missing %q:\n%s", want, stdout.String())
		}
	}
}

func TestContinueAliasJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "handoff.jsonl")
	fixture := `{"type":"user","timestamp":"2026-09-20T10:00:00Z","cwd":"/work/demo","sessionId":"session-1","message":{"role":"user","content":"continue this work"}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if err := Run([]string{"continue", path, "--format", "json"}, os.Stdin, &stdout, &stderr); err != nil {
		t.Fatalf("Run continue: %v", err)
	}
	var report transcript.HandoffReport
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatalf("decode handoff JSON: %v\n%s", err, stdout.String())
	}
	if report.Session.Project != "demo" || len(report.UserRequests) != 1 {
		t.Fatalf("unexpected handoff report: %+v", report)
	}
}
