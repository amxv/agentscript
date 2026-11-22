package transcript

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestParseCodexCompletedActivity(t *testing.T) {
	tr, err := ParseFile(filepath.Join("testdata", "codex-completed-activity.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	want := []struct {
		kind Kind
		name string
		text string
	}{
		{KindUser, "", "[Image attachment]"},
		{KindThinking, "", "Inspect the example."},
		{KindAssistant, "", "I will check the example."},
		{KindCommand, "exec", ""},
		{KindCommand, "Bash", ""},
		{KindCommandResult, "Bash", ""},
		{KindToolCall, "example", ""},
		{KindToolResult, "example", "dynamic output"},
		{KindToolCall, "Edit", ""},
		{KindToolResult, "Edit", "updated"},
		{KindToolCall, "mcp__example__lookup", ""},
		{KindToolResult, "mcp__example__lookup", "not found"},
		{KindCommandResult, "exec", "first line\nsecond line"},
		{KindToolResult, "example_selection", "selected example"},
		{KindAssistant, "", "Done."},
	}
	if len(tr.Blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d", len(tr.Blocks), len(want))
	}
	for index, expected := range want {
		block := tr.Blocks[index]
		if block.Index != index || block.Turn != 1 || block.Kind != expected.kind ||
			block.ToolName != expected.name || block.Text != expected.text {
			t.Errorf("block #%03d: got %+v, want %+v", index, block, expected)
		}
	}
	if cmd := tr.Blocks[4].ToolInput["command"]; cmd != "cd '/example folder' && exit 2" {
		t.Errorf("command = %q", cmd)
	}
	if !tr.Blocks[5].IsError || !tr.Blocks[11].IsError {
		t.Error("shell exit status or MCP error was lost")
	}
	if got := tr.Blocks[13].ToolID; got != "selection" {
		t.Errorf("standalone result ID = %q", got)
	}
	messages := FilterBlocks(tr.Blocks, RenderOptions{MessagesOnly: true})
	var indexes []int
	for _, block := range messages {
		indexes = append(indexes, block.Index)
	}
	if !slices.Equal(indexes, []int{0, 2, 14}) {
		t.Fatalf("message indexes = %v", indexes)
	}
}

func TestParseCodexMergesCompletedToolWithResponseItems(t *testing.T) {
	data := []byte(`{"type":"session_meta"}
{"type":"event_msg","payload":{"type":"task_started"}}
{"type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"first","arguments":"{\"cmd\":\"true\"}"}}
{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"CommandExecution","id":"first","command":["true"],"aggregated_output":"mirror"}}}
{"type":"response_item","payload":{"type":"function_call_output","call_id":"first","output":[{"type":"input_text","text":"original"}]}}
{"type":"response_item","payload":{"type":"function_call","name":"exec_command","call_id":"second","arguments":"{\"cmd\":\"printf hello\"}"}}
{"type":"event_msg","payload":{"type":"item_completed","item":{"type":"CommandExecution","id":"second","command":["printf","hello"],"aggregated_output":"event only"}}}
{"type":"response_item","payload":{"type":"reasoning","content":[{"type":"reasoning_text","text":"Readable reasoning."}],"summary":[]}}`)
	tr, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Blocks) != 5 {
		t.Fatalf("got %d blocks, want 5", len(tr.Blocks))
	}
	for i, want := range []string{"original", "event only"} {
		call, result := tr.Blocks[i*2], tr.Blocks[i*2+1]
		if call.Kind != KindCommand || result.Kind != KindCommandResult || result.ToolID != call.ToolID || result.Text != want {
			t.Errorf("incorrectly paired tool call and result: %+v %+v", call, result)
		}
	}
	if block := tr.Blocks[4]; block.Kind != KindThinking || block.Text != "Readable reasoning." {
		t.Errorf("unexpected reasoning block: %+v", block)
	}
}

func TestParseClaudeAttachments(t *testing.T) {
	tr, err := ParseFile(filepath.Join("testdata", "claude-attachments.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(tr.Blocks) != 7 {
		t.Fatalf("got %d blocks, want 7", len(tr.Blocks))
	}
	if block := tr.Blocks[0]; block.Kind != KindUser || block.Text != "Check this diagram.\n[Image attachment]" {
		t.Errorf("unexpected user block: %+v", block)
	}
	for _, index := range []int{2, 4, 6} {
		call, result := tr.Blocks[index-1], tr.Blocks[index]
		if result.Index != index || result.ToolID != call.ToolID || result.ToolName != call.ToolName {
			t.Errorf("incorrectly paired Claude tool call and result: %+v %+v", call, result)
		}
	}
	if tr.Blocks[2].Text != "[Image attachment]" || tr.Blocks[4].Text != "[Tool reference: mcp__example__lookup]" ||
		tr.Blocks[6].Kind != KindCommandResult || tr.Blocks[6].Text != "" {
		t.Error("image, tool reference, or empty command result was lost")
	}
	var rendered strings.Builder
	if err := Render(&rendered, tr, RenderOptions{}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered.String(), "AAA=") {
		t.Error("image data leaked into text output")
	}
}
