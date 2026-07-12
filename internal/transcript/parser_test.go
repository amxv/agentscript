package transcript

import (
	"os"
	"strings"
	"testing"
)

func TestParseClaudeFixture(t *testing.T) {
	data, err := os.ReadFile("../../tmp/claude-replay/test/fixture.jsonl")
	if err != nil {
		t.Skipf("reference fixture unavailable: %v", err)
	}
	tr, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tr.Provider != ProviderClaude {
		t.Fatalf("provider = %s, want claude", tr.Provider)
	}
	if len(tr.Blocks) < 9 {
		t.Fatalf("expected at least 9 blocks, got %d", len(tr.Blocks))
	}
	if tr.Blocks[0].Kind != KindUser || !strings.Contains(tr.Blocks[0].Text, "2+2") {
		t.Fatalf("unexpected first block: %#v", tr.Blocks[0])
	}
	if tr.Blocks[4].Kind != KindToolCall || tr.Blocks[4].ToolName != "Read" {
		t.Fatalf("expected Read tool call at #004, got %#v", tr.Blocks[4])
	}
	if tr.Blocks[5].Kind != KindToolResult || !strings.Contains(tr.Blocks[5].Text, "file contents") {
		t.Fatalf("expected tool result at #005, got %#v", tr.Blocks[5])
	}
}

func TestParseCodexFixture(t *testing.T) {
	data, err := os.ReadFile("../../tmp/claude-replay/test/fixture-codex.jsonl")
	if err != nil {
		t.Skipf("reference fixture unavailable: %v", err)
	}
	tr, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if tr.Provider != ProviderCodex {
		t.Fatalf("provider = %s, want codex", tr.Provider)
	}
	if tr.Blocks[2].Kind != KindCommand || tr.Blocks[2].ToolName != "Bash" {
		t.Fatalf("expected Bash command at #002, got %#v", tr.Blocks[2])
	}
	if !strings.Contains(tr.Blocks[2].ToolInput["command"].(string), "ls") {
		t.Fatalf("expected command input to include ls, got %#v", tr.Blocks[2].ToolInput)
	}
	if tr.Blocks[7].Kind != KindToolCall || tr.Blocks[7].ToolName != "Write" {
		t.Fatalf("expected Write call at #007, got %#v", tr.Blocks[7])
	}
}

func TestSliceBlocksKeepsOriginalIndexes(t *testing.T) {
	blocks := []Block{{Index: 0}, {Index: 1}, {Index: 2}, {Index: 3}}
	from, to := 1, 2
	got := SliceBlocks(blocks, SliceSpec{From: &from, To: &to}, 0, -1, 0, 0)
	if len(got) != 2 || got[0].Index != 1 || got[1].Index != 2 {
		t.Fatalf("unexpected slice: %#v", got)
	}
}

func TestFinishDeduplicatesCodexTransportCopies(t *testing.T) {
	eventRaw := []byte(`{"type":"event_msg"}`)
	responseRaw := []byte(`{"type":"response_item"}`)
	blocks := []Block{
		{Kind: KindUser, Text: "same\nmessage", Raw: eventRaw},
		{Kind: KindUser, Text: "same message", Raw: responseRaw},
		{Kind: KindThinking, Text: "preserved thinking"},
	}

	tr := finish(ProviderCodex, blocks)
	if len(tr.Blocks) != 2 || tr.Blocks[0].Text != "same\nmessage" || tr.Blocks[1].Kind != KindThinking {
		t.Fatalf("unexpected deduplicated transcript: %#v", tr.Blocks)
	}
}

func TestFinishDeduplicatesSameTimestampCodexCopies(t *testing.T) {
	raw := []byte(`{"type":"response_item"}`)
	blocks := []Block{
		{Kind: KindUser, Text: `<team_msg sender="agent">same message</team_msg>`, Timestamp: "2026-07-12T01:00:00.000Z", Raw: raw},
		{Kind: KindUser, Text: "<team_msg sender=\"agent\">\nsame message\n</team_msg>", Timestamp: "2026-07-12T01:00:00.000Z", Raw: raw},
	}

	tr := finish(ProviderCodex, blocks)
	if len(tr.Blocks) != 1 {
		t.Fatalf("got %d blocks, want one semantic team message: %#v", len(tr.Blocks), tr.Blocks)
	}
}

func TestParseClaudePreservesSemanticWrappers(t *testing.T) {
	data := []byte(`{"type":"user","message":{"role":"user","content":"<user_query>hello</user_query>\\n<task-notification>done</task-notification>\\n<command-name>/test</command-name>"}}`)
	tr, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(tr.Blocks) != 1 {
		t.Fatalf("got %d blocks, want 1", len(tr.Blocks))
	}
	for _, want := range []string{"<user_query>", "<task-notification>", "<command-name>"} {
		if !strings.Contains(tr.Blocks[0].Text, want) {
			t.Fatalf("wrapper %q was not preserved: %q", want, tr.Blocks[0].Text)
		}
	}
}
