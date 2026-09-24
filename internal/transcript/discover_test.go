package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCodexSessionID = "019f91bc-123f-7692-8a78-21e54d6677e6"

func TestResolveCodexSessionID(t *testing.T) {
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	sessionDir := filepath.Join(root, "2026", "07", "24")
	if err := os.MkdirAll(sessionDir, 0o755); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(sessionDir, "rollout-2026-07-24T06-57-29-"+testCodexSessionID+".jsonl")
	if err := os.WriteFile(want, []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveCodexSessionID(strings.ToUpper(testCodexSessionID), []string{root})
	if err != nil {
		t.Fatalf("ResolveCodexSessionID: %v", err)
	}
	if got != want {
		t.Fatalf("resolved path = %q, want %q", got, want)
	}
}

func TestResolveCodexSessionIDNotFound(t *testing.T) {
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	_, err := ResolveCodexSessionID(testCodexSessionID, []string{t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestResolveCodexSessionIDAmbiguous(t *testing.T) {
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	for _, dir := range []string{"one", "two"} {
		path := filepath.Join(root, dir)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		filename := filepath.Join(path, "rollout-"+testCodexSessionID+".jsonl")
		if err := os.WriteFile(filename, []byte("{}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, err := ResolveCodexSessionID(testCodexSessionID, []string{root})
	if err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("expected ambiguous error, got %v", err)
	}
}

func TestDefaultRootsUseEnvironmentConfigDirs(t *testing.T) {
	root := t.TempDir()
	claudeConfig := filepath.Join(root, "claude")
	codexHome := filepath.Join(root, "codex")
	claudeProjects := filepath.Join(claudeConfig, "projects")
	codexSessions := filepath.Join(codexHome, "sessions")
	for _, dir := range []string{claudeProjects, codexSessions} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("CLAUDE_CONFIG_DIR", claudeConfig)
	t.Setenv("CODEX_HOME", codexHome)

	roots := DefaultRoots()
	if len(roots) != 2 || roots[0] != claudeProjects || roots[1] != codexSessions {
		t.Fatalf("DefaultRoots() = %q, want [%q %q]", roots, claudeProjects, codexSessions)
	}
}

func TestDefaultRootsFallBackWhenEnvironmentDirsAreMissing(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", filepath.Join(root, "missing-claude"))
	t.Setenv("CODEX_HOME", filepath.Join(root, "missing-codex"))

	roots := DefaultRoots()
	if len(roots) != 2 || roots[0] != "~/.claude/projects" || roots[1] != "~/.codex/sessions" {
		t.Fatalf("DefaultRoots() = %q, want standard fallbacks", roots)
	}
}

func TestDiscoverKeepsProviderFromConfiguredSource(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".gg", "codex", "sessions")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"session_meta","payload":{"id":"abc","cwd":"/tmp/project"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sessions, err := DiscoverSources(0, ProviderCodex, []SessionSource{{Path: root, Provider: ProviderCodex}})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Provider != ProviderCodex {
		t.Fatalf("sessions = %#v, want one Codex session", sessions)
	}
}

func TestDiscoverDetectsProviderInArbitraryCustomRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "opaque.jsonl")
	fixture := `{"type":"user","message":{"role":"user","content":"hello"}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	sessions, err := Discover(0, ProviderUnknown, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 1 || sessions[0].Provider != ProviderClaude {
		t.Fatalf("sessions = %#v, want one Claude session", sessions)
	}
}

func TestHydrateSessionReadsProjectAndPrompt(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "claude.jsonl")
	fixture := `{"type":"user","timestamp":"2026-09-20T10:00:00Z","cwd":"/work/agentscript","sessionId":"session-1","message":{"role":"user","content":"add history search"}}` + "\n"
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	s := HydrateSession(Session{Path: path, Provider: ProviderClaude, ModTime: info.ModTime(), Size: info.Size(), Title: "fallback"})
	if s.ID != "session-1" || s.Project != "agentscript" || s.CWD != "/work/agentscript" || s.FirstPrompt != "add history search" {
		t.Fatalf("hydrated session = %#v", s)
	}
	if !strings.Contains(s.Title, "agentscript") || !strings.Contains(s.Title, "add history search") {
		t.Fatalf("title = %q", s.Title)
	}
}

func TestHydrateSessionFromTranscriptUsesFirstMeaningfulUserBlock(t *testing.T) {
	s := Session{Path: "/tmp/session.jsonl", Provider: ProviderCodex, CWD: "/work/agentscript", Title: "fallback"}
	tr := Transcript{Provider: ProviderCodex, Blocks: []Block{
		{Kind: KindUser, Text: "You were added to the workspace team \"agentscript\".", Timestamp: "2026-09-20T09:59:59Z"},
		{Kind: KindUser, Text: "<environment_context><cwd>/work/agentscript</cwd></environment_context>", Timestamp: "2026-09-20T10:00:00Z"},
		{Kind: KindUser, Text: "design transcript search", Timestamp: "2026-09-20T10:00:01Z"},
	}}
	s = HydrateSessionFromTranscript(s, tr)
	if s.FirstPrompt != "design transcript search" {
		t.Fatalf("FirstPrompt = %q", s.FirstPrompt)
	}
	if s.Project != "agentscript" || !strings.Contains(s.Title, "design transcript search") {
		t.Fatalf("hydrated session = %#v", s)
	}
	if s.StartedAt.IsZero() {
		t.Fatal("expected StartedAt from transcript block")
	}
}
