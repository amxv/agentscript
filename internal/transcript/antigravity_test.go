package transcript

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func agyFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "antigravity-"+name+".jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAntigravityNative(t *testing.T) {
	data := agyFixture(t, "native")
	for _, tail := range []string{"", "\n", "\n{\"type\":"} {
		tr, err := Parse(append(bytes.TrimSpace(data), tail...))
		if err != nil || tr.Provider != ProviderAntigravity {
			t.Fatal(tr.Provider, err)
		}
		kinds := []Kind{KindUser, KindThinking, KindAssistant, KindToolCall, KindCommand, KindToolResult, KindCommandResult, KindSystem, KindUser, KindToolCall, KindToolResult, KindAssistant}
		if len(tr.Blocks) != len(kinds) {
			t.Fatalf("got %d blocks", len(tr.Blocks))
		}
		for i, k := range kinds {
			if b := tr.Blocks[i]; b.Index != i || b.Kind != k {
				t.Fatalf("block %d = %+v", i, b)
			}
		}
		b := tr.Blocks
		if b[0].Text != "Inspect the diagram.\n"+imageAttachment || b[0].Timestamp == "" || b[5].ToolID != b[3].ToolID || b[6].ToolID != b[4].ToolID || !b[5].IsError || !b[6].IsError || b[10].Text != "" {
			t.Fatalf("incorrect native normalization: %+v", b)
		}
		refs := FileRefs(tr, true)
		if len(refs) != 1 || refs[0].Path != "example without extension" || refs[0].Ops[0] != "edit" {
			t.Fatal(refs)
		}
		commands, err := CommandEntries(tr, true, "", false, false, true)
		if err != nil || len(commands) != 1 || commands[0].Command != "false" || commands[0].ResultIndex != 6 {
			t.Fatal(commands, err)
		}
		tail := FilterBlocks(LastTurnBlocks(b, 1), RenderOptions{MessagesOnly: true})
		if len(tail) != 2 || tail[0].Index != 8 || tail[1].Index != 11 || tail[0].Turn != 2 {
			t.Fatal(tail)
		}
	}
	report, err := InspectCompactions(data)
	if err != nil || len(report.Compactions) != 1 {
		t.Fatal(report, err)
	}
	c := report.Compactions[0]
	if *c.Antigravity.StepIndex != 4 || c.RawEvents[0].Line != 5 || *c.PreTotalTokens != 12 || *c.ProviderPostTokens != 5 || c.Timestamp == "" {
		t.Fatal(c)
	}
}

func TestAntigravitySystemAndLegacyToolSteps(t *testing.T) {
	for _, kind := range []string{"CONVERSATION_HISTORY", "EPHEMERAL_MESSAGE", "SYSTEM_MESSAGE", "ERROR_MESSAGE"} {
		data := fmt.Appendf(nil, `{"step_index":1,"type":"%s","source":"SYSTEM","content":"detail"}`, kind)
		tr, err := Parse(data)
		if err != nil || len(tr.Blocks) != 1 || tr.Blocks[0].Kind != KindSystem || tr.Blocks[0].Text != "detail" {
			t.Fatal(kind, tr, err)
		}
	}
	for _, kind := range []string{"RUN_COMMAND", "LIST_DIRECTORY", "VIEW_FILE", "CODE_ACTION", "GREP_SEARCH", "INVOKE_SUBAGENT", "READ_URL_CONTENT", "SEARCH_WEB", "ASK_QUESTION"} {
		data := fmt.Appendf(nil, `{"step_index":1,"type":"%s","source":"MODEL","content":"example"}`, kind)
		tr, err := Parse(data)
		want := KindToolResult
		if kind == "RUN_COMMAND" {
			want = KindCommandResult
		}
		if err != nil || len(tr.Blocks) != 1 || tr.Blocks[0].Kind != want || tr.Blocks[0].Text != "example" {
			t.Fatal(kind, tr, err)
		}
	}
}

func TestAntigravityCaptures(t *testing.T) {
	tr, err := Parse(agyFixture(t, "acp"))
	if err != nil || len(tr.Blocks) != 9 {
		t.Fatal(tr, err)
	}
	b := tr.Blocks
	if !strings.HasSuffix(b[0].Text, imageAttachment) || b[1].Kind != KindThinking || b[3].Turn != 1 || b[4].Turn != 2 || b[3].ToolID == b[4].ToolID || b[5].ToolID != b[3].ToolID || !b[6].IsError || b[7].Text != "Goodbye" || b[8].Turn != 1 {
		t.Fatal(b)
	}
	tail := LastTurnBlocks(b, 1)
	if len(tail) != 4 || tail[len(tail)-1].Text != "Goodbye" {
		t.Fatal(tail)
	}
	stream := agyFixture(t, "stream")
	tr, err = Parse(stream)
	if err != nil || len(tr.Blocks) != 3 || tr.Blocks[2].Text != "All done" || tr.Blocks[2].Turn != 1 {
		t.Fatal(tr, err)
	}
	report, err := InspectCompactions(stream)
	if err != nil || len(report.Compactions) != 1 {
		t.Fatal(report, err)
	}
	tr, err = Parse([]byte("{\n\"conversation_id\":\"abc\",\n\"status\":\"ERROR\", \"error\":\"oops\", \"duration_seconds\":1\n}"))
	if err != nil || len(tr.Blocks) != 1 || tr.Blocks[0].Kind != KindSystem || !tr.Blocks[0].IsError {
		t.Fatal(tr, err)
	}
}

func TestAntigravityDiscovery(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("AGENTSCRIPT_CACHE_DIR", t.TempDir())
	brain := filepath.Join(home, ".gemini", "antigravity-cli", "brain")
	if root := antigravityRoot(); root != brain {
		t.Fatalf("%s root %s, want %s", runtime.GOOS, root, brain)
	}
	logs := agyLogDir(filepath.Join(brain, testCodexSessionID))
	write := func(path string, data []byte) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	full := filepath.Join(logs, "transcript_full.jsonl")
	short := filepath.Join(logs, "transcript.jsonl")
	write(short, agyFixture(t, "native"))
	write(filepath.Join(logs, "chunks", "transcript_full", "00000000.jsonl"), agyFixture(t, "native"))
	write(full, nil)
	project := filepath.Join(home, "example")
	uri := "file:///" + strings.TrimPrefix(filepath.ToSlash(project), "/")
	cache, _ := json.Marshal(map[string]any{"conversations": map[string]any{testCodexSessionID: map[string]any{"summary": map[string]any{"WorkspaceURIs": []string{uri}}}}})
	write(filepath.Join(filepath.Dir(brain), "cache", "conversation_metadata.json"), cache)
	assertPath := func(want string) {
		t.Helper()
		ss, err := DiscoverCached(0, ProviderAntigravity, nil, true)
		if err != nil || len(ss) != 1 || ss[0].Path != want || ss[0].CWD != project {
			t.Fatal(ss, err)
		}
		s := HydrateSession(ss[0])
		if s.ID != testCodexSessionID || !strings.HasPrefix(s.FirstPrompt, "Inspect") || s.StartedAt.IsZero() {
			t.Fatal(s)
		}
		path, err := ResolveSessionReference(testCodexSessionID, nil)
		if err != nil || path != want {
			t.Fatal(path, err)
		}
	}
	assertPath(short)
	write(full, agyFixture(t, "native"))
	assertPath(full)
}

func TestAntigravityHomeLayouts(t *testing.T) {
	for _, tc := range []struct{ os, home, want string }{
		{"darwin", "/Users/sample", "/Users/sample/.gemini/antigravity-cli/brain"},
		{"linux", "/home/sample", "/home/sample/.gemini/antigravity-cli/brain"},
		{"windows", `C:\Users\sample`, `C:\Users\sample\.gemini\antigravity-cli\brain`},
	} {
		t.Run(tc.os, func(t *testing.T) {
			t.Setenv("HOME", tc.home)
			t.Setenv("USERPROFILE", tc.home)
			got := transcriptPath(filepath.ToSlash(antigravityRoot()), tc.os == "windows")
			if got != tc.want {
				t.Fatalf("%s, want %s", got, tc.want)
			}
		})
	}
}

func TestAntigravityHeadlessToolError(t *testing.T) {
	data := []byte(`{"event":"step_update","step_update":{"conversation_id":"abc","step_index":2,"state":"DONE","step_type":"tool","tool_name":"read_file","tool_info":{"error":{"type":"missing","message":"no such file"}}}}`)
	tr, err := Parse(data)
	if err != nil || len(tr.Blocks) != 2 || !tr.Blocks[1].IsError || tr.Blocks[1].Text != "no such file" {
		t.Fatal(tr, err)
	}
}

func TestAntigravityRecentWorkspaceSummary(t *testing.T) {
	field := func(n byte, value []byte) []byte {
		header := binary.AppendUvarint([]byte{n<<3 | 2}, uint64(len(value)))
		return append(header, value...)
	}
	summary := append(field(1, []byte("Synthetic title")), field(9, field(1, []byte("file:///example/project")))...)
	entry := append(field(1, []byte(testCodexSessionID)), field(2, summary)...)
	data := append(field(1, entry), 10, 255) // incomplete next entry
	metadata := make(map[string]agyMetadata)
	loadAgySummaries(data, metadata)
	meta := metadata[testCodexSessionID].Summary
	if meta.Title != "Synthetic title" || len(meta.WorkspaceURIs) != 1 || meta.WorkspaceURIs[0] != "file:///example/project" {
		t.Fatal(meta)
	}
}

func TestAntigravityPermissionAndQuestion(t *testing.T) {
	data := []byte(`{"dir":"in","msg":{"result":{"agentInfo":{"name":"Antigravity ACP Server"}}}}
{"dir":"out","msg":{"id":1,"method":"session/prompt","params":{"sessionId":"a","prompt":[{"type":"text","text":"Ask first"}]}}}
{"dir":"in","msg":{"id":1,"method":"session/request_permission","params":{"sessionId":"a","toolCall":{"toolCallId":"x","name":"write_file","title":"Write example","args":{"Path":"example"}}}}}
{"dir":"out","msg":{"id":1,"result":{"outcome":{"optionId":"allow"}}}}
{"dir":"in","msg":{"method":"session/update","params":{"sessionId":"a","update":{"sessionUpdate":"tool_call_update","toolCallId":"x","name":"write_file","status":"completed","content":[{"type":"diff","path":"example","oldText":"old value","newText":"new value"}]}}}}
{"dir":"in","msg":{"id":2,"method":"elicitation/create","params":{"sessionId":"a","message":"Choose a color","requestedSchema":{"type":"object"}}}}
{"dir":"out","msg":{"id":2,"result":{"action":"accept","content":{"color":"blue"}}}}
{"dir":"in","msg":{"id":1,"result":{"stopReason":"cancelled"}}}`)
	tr, err := Parse(data)
	if err != nil || len(tr.Blocks) != 8 {
		t.Fatal(tr, err)
	}
	for i, text := range map[int]string{2: "Permission requested", 3: "allow", 4: "new value", 5: "Choose a color", 6: "blue", 7: "cancelled"} {
		if !strings.Contains(tr.Blocks[i].Text, text) {
			t.Fatal(i, tr.Blocks[i])
		}
	}
}
