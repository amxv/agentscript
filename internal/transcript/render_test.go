package transcript

import (
	"strings"
	"testing"
)

func TestFilterBlocksHidesCodexInternalGoalByDefault(t *testing.T) {
	blocks := []Block{
		{Index: 4, Kind: KindUser, Text: "before\n<codex_internal_goal>secret legacy goal</codex_internal_goal>\nafter"},
		{Index: 5, Kind: KindUser, Text: `<codex_internal_context source="goal">secret current goal</codex_internal_context>`},
		{Index: 6, Kind: KindAssistant, Text: "visible"},
	}

	got := FilterBlocks(blocks, RenderOptions{})
	if len(got) != 2 {
		t.Fatalf("got %d blocks, want 2: %#v", len(got), got)
	}
	if got[0].Index != 4 || strings.Contains(got[0].Text, "secret") || got[0].Text != "before\nafter" {
		t.Fatalf("internal goal was not stripped cleanly: %#v", got[0])
	}
	if got[1].Index != 6 {
		t.Fatalf("hidden goal-only block changed stable indexes: %#v", got)
	}
}

func TestFilterBlocksCanShowCodexInternalGoal(t *testing.T) {
	block := Block{Index: 9, Kind: KindUser, Text: `<codex_internal_context source="goal">secret goal</codex_internal_context>`}

	got := FilterBlocks([]Block{block}, RenderOptions{ShowInternalGoal: true})
	if len(got) != 1 || got[0].Text != block.Text {
		t.Fatalf("--show-internal-goal equivalent did not preserve content: %#v", got)
	}
}
