package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/amxv/agentscript/internal/transcript"
)

func TestTailSelectionAndContinuation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AGENTSCRIPT_CONFIG", filepath.Join(dir, "no-config.json"))
	path := filepath.Join(dir, "a file's transcript.jsonl")
	fixture := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"preamble"}]}}`,
		`{"type":"user","message":{"content":"first"}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"reply"}]}}`,
		`{"type":"user","message":{"content":"é < & 😀"}}`,
		`{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"hidden"},{"type":"text","text":"visible"}]}}`,
		`{"type":"user","message":{"content":"third"}}`,
	}, "\n")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if err := Run(args, nil, &stdout, &stderr); err != nil {
			t.Fatalf("%v: %v (%s)", args, err, &stderr)
		}
		return stdout.String()
	}
	decode := func(output string, want []int) (int, string) {
		t.Helper()
		var page struct {
			Blocks []transcript.Block
			Next   *struct {
				Index   int
				Command string
			}
		}
		if err := json.Unmarshal([]byte(output), &page); err != nil {
			t.Fatalf("invalid JSON: %v\n%s", err, output)
		}
		var indexes []int
		for _, block := range page.Blocks {
			indexes = append(indexes, block.Index)
		}
		if !slices.Equal(indexes, want) {
			t.Fatalf("indexes = %v, want %v", indexes, want)
		}
		if page.Next == nil {
			return -1, ""
		}
		return page.Next.Index, page.Next.Command
	}
	for _, command := range []string{"open", "slice"} {
		decode(run(command, "--last-turns", "2", path, "--hide-thinking", "--format", "json"), []int{3, 5, 6})
		decode(run(command, path, "--last", "3", "--hide-thinking", "--format", "json"), []int{5, 6})
		decode(run(command, path, "--last-turns", "9", "--format", "json"), []int{1, 2, 3, 4, 5, 6})
	}

	first := run("open", path, "--slice", "3:3", "--format", "json")
	budget := strconv.Itoa(utf8.RuneCountInString(first))
	next, command := decode(run("slice", "--max-chars", budget, path, "3:5", "--hide-thinking", "--format", "json"), []int{3})
	wantCommand := "agentscript open --path " + commandQuote(path) + " --slice 5:5 --max-chars " + budget + " --format 'json' --hide-thinking=true"
	if next != 5 || command != wantCommand {
		t.Fatalf("next = %d, %q, want 5, %q", next, command, wantCommand)
	}
	if next, _ := decode(run("open", path, "--slice", "5:5", "--max-chars", budget, "--format", "json"), []int{5}); next != -1 {
		t.Fatalf("final page has next index %d", next)
	}
	next, command = decode(run("open", path, "--slice", "3:3", "--format", "json", "--max-chars", "1"), nil)
	if next != 3 || !strings.Contains(command, "--max-chars "+budget+" ") {
		t.Fatalf("oversized first block continuation = %d, %q", next, command)
	}
}
