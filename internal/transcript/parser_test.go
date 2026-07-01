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
