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
	if len(got) != 3 {
		t.Fatalf("got %d blocks, want 3: %#v", len(got), got)
	}
	if got[0].Index != 4 || strings.Contains(got[0].Text, "secret") || got[0].Text != "before\nafter" {
		t.Fatalf("internal goal was not stripped cleanly: %#v", got[0])
	}
	if got[1].Index != 5 || got[1].Text != "" || got[2].Index != 6 {
		t.Fatalf("goal-only block or stable indexes were not preserved: %#v", got)
	}
}

func TestFilterBlocksHidesOnlyInternalScaffolding(t *testing.T) {
	blocks := []Block{
		{Index: 1, Kind: KindUser, Text: "# AGENTS.md instructions for /repo\n<INSTRUCTIONS>secret</INSTRUCTIONS>"},
		{Index: 2, Kind: KindUser, Text: "before\n<environment_context><cwd>/repo</cwd></environment_context>\nafter"},
		{Index: 3, Kind: KindUser, Text: `<team_msg sender="agent">keep me</team_msg>`},
		{Index: 4, Kind: KindUser, Text: `<image path="x.png"></image>`},
		{Index: 5, Kind: KindUser, Text: `<user_query>keep query wrapper</user_query>`},
		{Index: 6, Kind: KindUser, Text: `<task-notification>keep task wrapper</task-notification>`},
		{Index: 7, Kind: KindUser, Text: `<command-name>keep command wrapper</command-name>`},
		{Index: 8, Kind: KindThinking, Text: "keep thinking"},
	}

	got := FilterBlocks(blocks, RenderOptions{})
	if len(got) != 7 || got[0].Text != "before\nafter" {
		t.Fatalf("unexpected internal filtering: %#v", got)
	}
	for _, want := range []string{"team_msg", "<image", "user_query", "task-notification", "command-name", "keep thinking"} {
		found := false
		for _, block := range got {
			found = found || strings.Contains(block.Text, want)
		}
		if !found {
			t.Fatalf("preserved content %q missing from %#v", want, got)
		}
	}

	shown := FilterBlocks(blocks, RenderOptions{ShowInternal: true})
	if len(shown) != len(blocks) || !strings.Contains(shown[0].Text, "secret") {
		t.Fatalf("--show-internal did not restore scaffolding: %#v", shown)
	}
}

func TestFilterBlocksCanShowCodexInternalGoal(t *testing.T) {
	block := Block{Index: 9, Kind: KindUser, Text: `<codex_internal_context source="goal">secret goal</codex_internal_context>`}

	got := FilterBlocks([]Block{block}, RenderOptions{ShowInternalGoal: true})
	if len(got) != 1 || got[0].Text != block.Text {
		t.Fatalf("--show-internal-goal equivalent did not preserve content: %#v", got)
	}
}
