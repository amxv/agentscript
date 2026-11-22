package transcript

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveSessionReferenceAcceptsFileURL(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "folder with spaces")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "session%20with spaces.jsonl")
	if err := os.WriteFile(path, []byte(`{"type":"user","message":{"role":"user","content":"hello"}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	urlPath := filepath.ToSlash(path)
	if !strings.HasPrefix(urlPath, "/") {
		urlPath = "/" + urlPath
	}
	ref := (&url.URL{Scheme: "file", Path: urlPath}).String()
	got, err := ResolveSessionReference(ref, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got != path {
		t.Fatalf("resolved = %q, want %q", got, path)
	}
}

func TestTranscriptPathSyntax(t *testing.T) {
	tests := []struct {
		input   string
		windows bool
		want    string
	}{
		{`C:\logs\session.jsonl`, true, `C:\logs\session.jsonl`},
		{`C:/logs/session.jsonl`, true, `C:\logs\session.jsonl`},
		{`\\?\C:\logs\session.jsonl`, true, `\\?\C:\logs\session.jsonl`},
		{`\\?\UNC\server\share\session.jsonl`, true, `\\?\UNC\server\share\session.jsonl`},
		{"file:///C:/logs/my%20session.jsonl", true, `C:\logs\my session.jsonl`},
		{"file://localhost/C:/logs/session.jsonl", true, `C:\logs\session.jsonl`},
		{"file://C:/logs/session.jsonl", true, `C:\logs\session.jsonl`},
		{"file://server/share/session.jsonl", true, `\\server\share\session.jsonl`},
		{"file:////%3F/C:/logs/session.jsonl", true, `\\?\C:\logs\session.jsonl`},
		{"file:///tmp/a%2520b.jsonl", false, "/tmp/a%20b.jsonl"},
		{"file:///C:/logs/session.jsonl", false, "/C:/logs/session.jsonl"},
		{`/tmp/a\b.jsonl`, false, `/tmp/a\b.jsonl`},
		{"019f91bc-123f-7692-8a78-21e54d6677e6", true, "019f91bc-123f-7692-8a78-21e54d6677e6"},
	}
	for _, tt := range tests {
		if got := transcriptPath(tt.input, tt.windows); got != tt.want {
			t.Errorf("transcriptPath(%q, %t) = %q, want %q", tt.input, tt.windows, got, tt.want)
		}
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
