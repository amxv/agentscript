package transcript

import (
	"encoding/json"
	"math"
	"testing"
)

func TestInspectClaudeCompactionsCorrelatesFullUsage(t *testing.T) {
	data := []byte(`{"type":"user","timestamp":"2026-08-03T02:00:00.000Z","message":{"role":"user","content":"hello"}}
{"type":"assistant","timestamp":"2026-08-03T02:03:00.000Z","message":{"role":"assistant","id":"before","usage":{"input_tokens":2,"cache_creation_input_tokens":100,"cache_read_input_tokens":200,"output_tokens":10},"content":[]}}
{"type":"system","timestamp":"2026-08-03T02:04:43.729Z","subtype":"compact_boundary","uuid":"boundary","compactMetadata":{"trigger":"manual","preTokens":312,"postTokens":78,"durationMs":92137,"cumulativeDroppedTokens":234,"preservedSegment":{"headUuid":"head","anchorUuid":"anchor","tailUuid":"tail"},"preservedMessages":{"anchorUuid":"anchor","uuids":["kept"],"allUuids":["kept","dropped"]}}}
{"type":"assistant","timestamp":"2026-08-03T02:04:44.000Z","message":{"role":"assistant","id":"synthetic","usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0},"content":[]}}
{"type":"assistant","timestamp":"2026-08-03T02:05:21.514Z","message":{"role":"assistant","id":"after","usage":{"input_tokens":2,"cache_creation_input_tokens":20,"cache_read_input_tokens":30,"output_tokens":4},"content":[]}}`)

	report, err := InspectCompactions(data)
	if err != nil {
		t.Fatalf("InspectCompactions: %v", err)
	}
	if report.Provider != ProviderClaude || len(report.Compactions) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	item := report.Compactions[0]
	if item.Claude == nil || item.Claude.PreTokens == nil || *item.Claude.PreTokens != 312 {
		t.Fatalf("missing Claude metadata: %#v", item.Claude)
	}
	if item.ProviderPostTokens == nil || *item.ProviderPostTokens != 78 || item.ProviderPostTokensSemantics != "payload_only" {
		t.Fatalf("unexpected provider post tokens: %#v", item)
	}
	if item.FirstFullUsageAfter == nil || item.FirstFullUsageAfter.MessageID != "after" {
		t.Fatalf("synthetic usage was not skipped: %#v", item.FirstFullUsageAfter)
	}
	if item.FirstFullUsageAfter.ComputedTotalTokens == nil || *item.FirstFullUsageAfter.ComputedTotalTokens != 56 {
		t.Fatalf("computed usage total = %#v, want 56", item.FirstFullUsageAfter.ComputedTotalTokens)
	}
	if item.Claude.PreservedMessages == nil || item.Claude.PreservedMessages.Count != 1 || item.Claude.PreservedMessages.AllCount != 2 {
		t.Fatalf("preserved message counts missing: %#v", item.Claude.PreservedMessages)
	}
	if len(item.RawEvents) != 1 || item.RawEvents[0].Line != 3 {
		t.Fatalf("unexpected raw event refs: %#v", item.RawEvents)
	}
}

func TestInspectCodexCompactionsPairsLifecycleRecords(t *testing.T) {
	data := []byte(`{"type":"session_meta","timestamp":"2026-08-03T01:00:00.000Z","payload":{"id":"session"}}
{"type":"event_msg","timestamp":"2026-08-03T01:01:00.000Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":80,"cache_write_input_tokens":0,"output_tokens":10,"reasoning_output_tokens":2,"total_tokens":110},"last_token_usage":{"input_tokens":100,"cached_input_tokens":80,"cache_write_input_tokens":0,"output_tokens":10,"reasoning_output_tokens":2,"total_tokens":110},"model_context_window":1000}}}
{"type":"compacted","timestamp":"2026-08-03T01:02:00.000Z","payload":{"window_id":"new","previous_window_id":"old","first_window_id":"first","window_number":1,"replacement_history":[{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]},{"type":"compaction","id":"cmp","encrypted_content":"secret"}]}}
{"type":"event_msg","timestamp":"2026-08-03T01:02:00.010Z","payload":{"type":"token_count","info":{"total_token_usage":{"input_tokens":100,"cached_input_tokens":80,"cache_write_input_tokens":0,"output_tokens":10,"reasoning_output_tokens":2,"total_tokens":110},"last_token_usage":{"input_tokens":0,"cached_input_tokens":0,"cache_write_input_tokens":0,"output_tokens":0,"reasoning_output_tokens":0,"total_tokens":42},"model_context_window":1000}}}
{"type":"event_msg","timestamp":"2026-08-03T01:02:00.020Z","payload":{"type":"context_compacted"}}`)

	report, err := InspectCompactions(data)
	if err != nil {
		t.Fatalf("InspectCompactions: %v", err)
	}
	if report.Provider != ProviderCodex || len(report.Compactions) != 1 {
		t.Fatalf("unexpected report: %#v", report)
	}
	item := report.Compactions[0]
	if item.Codex == nil || item.Codex.WindowID != "new" || item.Codex.ReplacementHistoryCount != 2 {
		t.Fatalf("missing Codex metadata: %#v", item.Codex)
	}
	if item.PreTotalTokens == nil || *item.PreTotalTokens != 110 || item.ProviderPostTokens == nil || *item.ProviderPostTokens != 42 {
		t.Fatalf("unexpected normalized token fields: %#v", item)
	}
	if item.ContextWindow == nil || *item.ContextWindow != 1000 || item.RemainingPercentage == nil || math.Abs(*item.RemainingPercentage-95.8) > 0.001 {
		t.Fatalf("unexpected context summary: %#v", item)
	}
	if item.FirstFullUsageAfter == nil || item.FirstFullUsageAfter.LastUsage == nil || item.FirstFullUsageAfter.LastUsage.TotalTokens == nil || *item.FirstFullUsageAfter.LastUsage.TotalTokens != 42 {
		t.Fatalf("missing Codex last usage: %#v", item.FirstFullUsageAfter)
	}
	if len(item.RawEvents) != 2 || item.RawEvents[1].PayloadType != "context_compacted" {
		t.Fatalf("context boundary was not paired: %#v", item.RawEvents)
	}
}

func TestRenderCompactionsJSONIsMachineReadable(t *testing.T) {
	report := CompactionReport{Provider: ProviderClaude, Compactions: []Compaction{{Index: 0, Timestamp: "2026-08-03T00:00:00Z", Provider: ProviderClaude}}}
	var output []byte
	buffer := &byteBuffer{}
	if err := RenderCompactions(buffer, report, "json"); err != nil {
		t.Fatalf("RenderCompactions: %v", err)
	}
	output = buffer.Bytes()
	var decoded CompactionReport
	if err := json.Unmarshal(output, &decoded); err != nil {
		t.Fatalf("JSON output is invalid: %v", err)
	}
	if len(decoded.Compactions) != 1 || decoded.Provider != ProviderClaude {
		t.Fatalf("unexpected decoded report: %#v", decoded)
	}
}

type byteBuffer struct{ data []byte }

func (b *byteBuffer) Write(p []byte) (int, error) {
	b.data = append(b.data, p...)
	return len(p), nil
}

func (b *byteBuffer) Bytes() []byte { return b.data }
