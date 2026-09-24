package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandoffRecoversTaskBeforeContinueTurns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "claude.jsonl")
	fixture := strings.Join([]string{
		`{"type":"user","timestamp":"2026-09-20T10:00:00Z","cwd":"/work/demo","sessionId":"s1","message":{"role":"user","content":"package 15 airports"}}`,
		`{"type":"assistant","timestamp":"2026-09-20T10:00:01Z","message":{"role":"assistant","content":[{"type":"text","text":"working"}]}}`,
		`{"type":"user","timestamp":"2026-09-20T10:00:02Z","cwd":"/work/demo","sessionId":"s1","message":{"role":"user","content":"also remove the author line"}}`,
		`{"type":"assistant","timestamp":"2026-09-20T10:00:03Z","message":{"role":"assistant","content":[{"type":"tool_use","id":"t1","name":"Bash","input":{"command":"echo package"}}]}}`,
		`{"type":"user","timestamp":"2026-09-20T10:00:04Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"package"}]}}`,
		`{"type":"user","timestamp":"2026-09-20T10:00:05Z","cwd":"/work/demo","sessionId":"s1","message":{"role":"user","content":"keep going"}}`,
		`{"type":"user","timestamp":"2026-09-20T10:00:06Z","cwd":"/work/demo","sessionId":"s1","message":{"role":"user","content":"continue"}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := BuildHandoff(path, HandoffOptions{LastBlocks: 40, UserTurns: 3, MaxLines: 30, MaxFiles: 20, MaxCommands: 10, IncludeRecent: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.UserRequests) != 4 {
		t.Fatalf("requests = %#v, want task + modifier + two continuation turns", report.UserRequests)
	}
	if report.UserRequests[0].Text != "package 15 airports" || report.UserRequests[1].Text != "also remove the author line" {
		t.Fatalf("unexpected request context: %#v", report.UserRequests)
	}
	if report.AnchorIndex != report.UserRequests[1].Index {
		t.Fatalf("anchor = %d, want latest substantive request %d", report.AnchorIndex, report.UserRequests[1].Index)
	}
	for _, block := range report.RecentBlocks {
		if block.Index < report.AnchorIndex {
			t.Fatalf("recent block #%d predates task anchor #%d", block.Index, report.AnchorIndex)
		}
	}
}

func TestCommandSummaryUnwrapsCodeModeCommand(t *testing.T) {
	block := Block{Kind: KindCommand, ToolName: "exec", ToolInput: map[string]any{
		"raw": `const r=await tools.exec_command({cmd:"go test ./...",workdir:"/repo"}); text(r.output);`,
	}}
	if got := commandSummaryText(block); got != "go test ./..." {
		t.Fatalf("commandSummaryText = %q", got)
	}
}

func TestLooksLikeFailureAvoidsIncidentalFailedWord(t *testing.T) {
	if looksLikeFailure("Documentation: a request is retried when it previously failed validation.") {
		t.Fatal("incidental prose should not be classified as a failure")
	}
	if !looksLikeFailure("Error: command could not start") {
		t.Fatal("explicit error should be classified as a failure")
	}
}

func TestAuxiliarySessionClassification(t *testing.T) {
	if got := auxiliarySessionKind("You are helping with git commits. Return ONLY valid JSON"); got != "git commit helper" {
		t.Fatalf("git helper kind = %q", got)
	}
	if got := auxiliarySessionKind("You were added to the workspace team \"demo\"."); got != "" {
		t.Fatalf("workspace agent should remain a normal searchable work session: %q", got)
	}
	if got := auxiliarySessionKind("Implement the search feature"); got != "" {
		t.Fatalf("normal prompt classified as auxiliary: %q", got)
	}
}
