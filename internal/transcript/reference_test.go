package transcript

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSessionReferenceAcceptsFileURL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "folder with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "session.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"user","message":{"role":"user","content":"hello"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ref := (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	got, err := ResolveSessionReference(ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("resolved = %q, want %q", got, path)
	}
}

func TestResolveSessionReferenceFindsClaudeSessionID(t *testing.T) {
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	const id = "019f91bc-123f-7692-8a78-21e54d6677e6"
	root := t.TempDir()
	path := filepath.Join(root, id+".jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"user","message":{"role":"user","content":"hello"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := ResolveSessionReference(id, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("resolved = %q, want %q", got, path)
	}
}
