package transcript

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testCodexSessionID = "019f91bc-123f-7692-8a78-21e54d6677e6"

func TestResolveCodexSessionID(t *testing.T) {
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
	_, err := ResolveCodexSessionID(testCodexSessionID, []string{t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "was not found") {
		t.Fatalf("expected not found error, got %v", err)
	}
}

func TestResolveCodexSessionIDAmbiguous(t *testing.T) {
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
