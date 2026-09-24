package transcript

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSearchDefaultsToAllQueries(t *testing.T) {
	blocks := []Block{
		{Index: 0, Kind: KindUser, Text: "push completed"},
		{Index: 1, Kind: KindAssistant, Text: "push rejected by remote"},
	}
	s := Session{Path: "/tmp/session", Provider: ProviderClaude}
	matches := searchTranscript(s, blocks, SearchOptions{Queries: []string{"push", "rejected"}, Mode: SearchAll}, nil)
	if len(matches) != 1 || matches[0].Block.Index != 1 {
		t.Fatalf("matches = %#v, want block 1 only", matches)
	}
	matches = searchTranscript(s, blocks, SearchOptions{Queries: []string{"push", "rejected"}, Mode: SearchAny}, nil)
	if len(matches) != 2 {
		t.Fatalf("--any matches = %d, want 2", len(matches))
	}
}

func TestNearMatchesAreAnchoredOnActualHits(t *testing.T) {
	blocks := []Block{
		{Index: 10, Kind: KindUser, Text: "push failed"},
		{Index: 11, Kind: KindAssistant, Text: "unrelated"},
		{Index: 14, Kind: KindCommandResult, Text: "remote rejected update"},
		{Index: 30, Kind: KindAssistant, Text: "rejected elsewhere"},
	}
	s := Session{Path: "/tmp/session", Provider: ProviderCodex}
	opts := SearchOptions{Queries: []string{"push", "rejected"}, Mode: SearchAll, Near: 5}
	matches := searchTranscript(s, blocks, opts, nil)
	if len(matches) != 1 {
		t.Fatalf("matches = %#v, want one proximity span", matches)
	}
	if matches[0].Block.Index != 10 || matches[0].EndIndex != 14 {
		t.Fatalf("span = %d:%d, want 10:14", matches[0].Block.Index, matches[0].EndIndex)
	}
}

func TestSearchHistoryCachesAndInvalidatesChangedTranscript(t *testing.T) {
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	// Exercise the pure-Go candidate fallback; ripgrep is an optional accelerator.
	t.Setenv("PATH", "")
	root := t.TempDir()
	path := filepath.Join(root, "session.jsonl")
	writeClaudeSearchFixture(t, path, "hello cache")
	info, _ := os.Stat(path)
	sessions := []Session{{Path: path, Provider: ProviderClaude, ModTime: info.ModTime(), Size: info.Size(), Title: "session"}}

	matches, stats, err := SearchHistory(sessions, SearchOptions{Queries: []string{"hello"}, Mode: SearchAll})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || stats.Parsed != 1 || stats.CacheHits != 0 {
		t.Fatalf("first search stats=%+v matches=%d", stats, len(matches))
	}

	matches, stats, err = SearchHistory(sessions, SearchOptions{Queries: []string{"hello"}, Mode: SearchAll})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || stats.CacheHits != 1 || stats.Parsed != 0 {
		t.Fatalf("cached search stats=%+v matches=%d", stats, len(matches))
	}

	time.Sleep(5 * time.Millisecond)
	writeClaudeSearchFixture(t, path, "goodbye cache")
	matches, stats, err = SearchHistory(sessions, SearchOptions{Queries: []string{"goodbye"}, Mode: SearchAll})
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 || stats.Parsed != 1 {
		t.Fatalf("changed search stats=%+v matches=%d", stats, len(matches))
	}
}

func writeClaudeSearchFixture(t *testing.T, path, text string) {
	t.Helper()
	fixture := `{"type":"user","timestamp":"2026-09-20T10:00:00Z","cwd":"/work/project","sessionId":"s1","message":{"role":"user","content":"` + text + `"}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestGroupMatchesRanksNormalSessionsBeforeAuxiliary(t *testing.T) {
	now := time.Now()
	matches := []Match{
		{Session: Session{Path: "/aux", ModTime: now, Auxiliary: "git commit helper"}, Block: Block{Index: 1}},
		{Session: Session{Path: "/normal", ModTime: now.Add(-time.Hour)}, Block: Block{Index: 2}},
	}
	groups := GroupMatches(matches)
	if len(groups) != 2 || groups[0].Session.Path != "/normal" {
		t.Fatalf("group order = %#v", groups)
	}
}
