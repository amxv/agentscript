package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAntigravityCommands(t *testing.T) {
	root := t.TempDir()
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	t.Setenv("AGENTSCRIPT_CONFIG", filepath.Join(root, "missing-config"))
	id := "00000000-0000-4000-8000-000000000001"
	logs := filepath.Join(root, "brain", id, ".system_generated", "logs")
	if err := os.MkdirAll(logs, 0755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(logs, "transcript_full.jsonl")
	data, err := os.ReadFile(filepath.Join("..", "transcript", "testdata", "antigravity-native.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		var out, stderr bytes.Buffer
		if err := Run(args, nil, &out, &stderr); err != nil {
			t.Fatalf("%q: %v (%s)", args, err, &stderr)
		}
		return out.String()
	}
	for _, args := range [][]string{
		{"open", "--latest", "1", "--provider", "antigravity", "--roots", root, "--last-turns", "1"},
		{"antigravity", id, "--roots", root, "--last", "1"},
		{"slice", "--turn-slice", "2:2", path, "--messages-only"},
		{"slice", path, "11:11"},
	} {
		out := run(args...)
		if !strings.Contains(out, "#011") || !strings.Contains(out, "All done.") || strings.Contains(out, "#000") {
			t.Fatal(args, out)
		}
	}
	for _, format := range []string{"text", "md", "html", "json"} {
		out := run("open", path, "--format", format, "--hide-thinking")
		if !strings.Contains(out, "All done.") || strings.Contains(out, "Inspect first") {
			t.Fatal(format, out)
		}
	}
	if out := run("handoff", id, "--roots", root); !strings.Contains(out, "Rewrite the example") {
		t.Fatal(out)
	}
	if out := run("open", "--no-tui", "--provider", "antigravity", "--roots", root); !strings.Contains(out, "antigravity") {
		t.Fatal(out)
	}
	if out := run("list", "--provider", "antigravity", "--roots", root); !strings.Contains(out, "antigravity") {
		t.Fatal(out)
	}
	for range 2 {
		out := run("search", "--roots", root, "diagram", "--provider", "antigravity", "--format", "json")
		if !strings.Contains(out, "Inspect the diagram.") {
			t.Fatal(out)
		}
	}
	var page struct {
		Next struct {
			Index   int
			Command string
		}
	}
	if err := json.Unmarshal([]byte(run("open", path, "--max-chars", "1", "--format", "json")), &page); err != nil || page.Next.Index != 0 || !strings.Contains(page.Next.Command, path) {
		t.Fatal(page, err)
	}
	if out := run("compactions", path); !strings.Contains(out, "CHECKPOINT step=4") {
		t.Fatal(out)
	}
}
