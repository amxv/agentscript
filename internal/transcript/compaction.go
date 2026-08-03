package transcript

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// CompactionReport is the machine-readable lifecycle view of a transcript.
// Blocks intentionally remain separate from this report: compaction and usage
// records are provider lifecycle events, not renderable conversation blocks.
type CompactionReport struct {
	Path        string       `json:"path,omitempty"`
	Provider    Provider     `json:"provider"`
	Compactions []Compaction `json:"compactions"`
}

type Compaction struct {
	Index                       int               `json:"index"`
	Timestamp                   string            `json:"timestamp"`
	Provider                    Provider          `json:"provider"`
	DurationMs                  *int64            `json:"duration_ms,omitempty"`
	PreTotalTokens              *int64            `json:"pre_total_tokens,omitempty"`
	ProviderPostTokens          *int64            `json:"provider_post_tokens,omitempty"`
	ProviderPostTokensSemantics string            `json:"provider_post_tokens_semantics,omitempty"`
	ProviderPostTokensSource    string            `json:"provider_post_tokens_source,omitempty"`
	ContextWindow               *int64            `json:"context_window,omitempty"`
	RemainingPercentage         *float64          `json:"remaining_percentage,omitempty"`
	Claude                      *ClaudeCompaction `json:"claude,omitempty"`
	Codex                       *CodexCompaction  `json:"codex,omitempty"`
	NearestUsageBefore          *UsageSnapshot    `json:"nearest_usage_before,omitempty"`
	NearestUsageAfter           *UsageSnapshot    `json:"nearest_usage_after,omitempty"`
	FirstFullUsageAfter         *UsageSnapshot    `json:"first_full_usage_after,omitempty"`
	RawEvents                   []RawEventRef     `json:"raw_events"`
}

type ClaudeCompaction struct {
	Trigger                 string                   `json:"trigger,omitempty"`
	PreTokens               *int64                   `json:"pre_tokens,omitempty"`
	PostTokens              *int64                   `json:"post_tokens,omitempty"`
	DurationMs              *int64                   `json:"duration_ms,omitempty"`
	CumulativeDroppedTokens *int64                   `json:"cumulative_dropped_tokens,omitempty"`
	PreservedSegment        *ClaudePreservedSegment  `json:"preserved_segment,omitempty"`
	PreservedMessages       *ClaudePreservedMessages `json:"preserved_messages,omitempty"`
}

type ClaudePreservedSegment struct {
	HeadUUID   string `json:"head_uuid,omitempty"`
	AnchorUUID string `json:"anchor_uuid,omitempty"`
	TailUUID   string `json:"tail_uuid,omitempty"`
}

type ClaudePreservedMessages struct {
	AnchorUUID string   `json:"anchor_uuid,omitempty"`
	UUIDs      []string `json:"uuids,omitempty"`
	AllUUIDs   []string `json:"all_uuids,omitempty"`
	Count      int      `json:"count"`
	AllCount   int      `json:"all_count"`
}

type CodexCompaction struct {
	WindowID                string             `json:"window_id,omitempty"`
	PreviousWindowID        string             `json:"previous_window_id,omitempty"`
	FirstWindowID           string             `json:"first_window_id,omitempty"`
	WindowNumber            *int64             `json:"window_number,omitempty"`
	ReplacementHistory      []CodexReplacement `json:"replacement_history,omitempty"`
	ReplacementHistoryCount int                `json:"replacement_history_count"`
}

type CodexReplacement struct {
	Type                        string         `json:"type,omitempty"`
	ID                          string         `json:"id,omitempty"`
	Role                        string         `json:"role,omitempty"`
	ContentCount                int            `json:"content_count,omitempty"`
	ContentTypes                []string       `json:"content_types,omitempty"`
	EncryptedContentPresent     bool           `json:"encrypted_content_present,omitempty"`
	EncryptedContentLength      int            `json:"encrypted_content_length,omitempty"`
	InternalChatMessageMetadata map[string]any `json:"internal_chat_message_metadata_passthrough,omitempty"`
}

// UsageSnapshot flattens the common usage fields while retaining Codex's
// cumulative and per-request snapshots in TotalUsage and LastUsage.
type UsageSnapshot struct {
	Timestamp                string            `json:"timestamp"`
	Line                     int               `json:"line"`
	RecordIndex              int               `json:"record_index"`
	Provider                 Provider          `json:"provider"`
	Source                   string            `json:"source,omitempty"`
	RawType                  string            `json:"raw_type,omitempty"`
	PayloadType              string            `json:"payload_type,omitempty"`
	MessageID                string            `json:"message_id,omitempty"`
	InputTokens              *int64            `json:"input_tokens,omitempty"`
	OutputTokens             *int64            `json:"output_tokens,omitempty"`
	CacheCreationInputTokens *int64            `json:"cache_creation_input_tokens,omitempty"`
	CacheReadInputTokens     *int64            `json:"cache_read_input_tokens,omitempty"`
	CachedInputTokens        *int64            `json:"cached_input_tokens,omitempty"`
	CacheWriteInputTokens    *int64            `json:"cache_write_input_tokens,omitempty"`
	ReasoningOutputTokens    *int64            `json:"reasoning_output_tokens,omitempty"`
	TotalTokens              *int64            `json:"total_tokens,omitempty"`
	ComputedTotalTokens      *int64            `json:"computed_total_tokens,omitempty"`
	ModelContextWindow       *int64            `json:"model_context_window,omitempty"`
	RemainingPercentage      *float64          `json:"remaining_percentage,omitempty"`
	TotalUsage               *TokenUsageValues `json:"total_usage,omitempty"`
	LastUsage                *TokenUsageValues `json:"last_usage,omitempty"`
}

type TokenUsageValues struct {
	InputTokens           *int64 `json:"input_tokens,omitempty"`
	CachedInputTokens     *int64 `json:"cached_input_tokens,omitempty"`
	CacheWriteInputTokens *int64 `json:"cache_write_input_tokens,omitempty"`
	OutputTokens          *int64 `json:"output_tokens,omitempty"`
	ReasoningOutputTokens *int64 `json:"reasoning_output_tokens,omitempty"`
	TotalTokens           *int64 `json:"total_tokens,omitempty"`
}

type RawEventRef struct {
	Line        int    `json:"line"`
	RecordIndex int    `json:"record_index"`
	Timestamp   string `json:"timestamp,omitempty"`
	Type        string `json:"type"`
	Subtype     string `json:"subtype,omitempty"`
	PayloadType string `json:"payload_type,omitempty"`
	UUID        string `json:"uuid,omitempty"`
}

type compactionCandidate struct {
	position int
	entry    jsonLine
	refs     []RawEventRef
	claude   *ClaudeCompaction
	codex    *CodexCompaction
}

type usageCandidate struct {
	position int
	snapshot UsageSnapshot
}

func InspectCompactionsFile(path string) (CompactionReport, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return CompactionReport{}, err
	}
	report, err := InspectCompactions(data)
	if err != nil {
		return CompactionReport{}, err
	}
	report.Path = path
	return report, nil
}

func InspectCompactions(data []byte) (CompactionReport, error) {
	entries, err := readJSONLines(data)
	if err != nil {
		return CompactionReport{}, err
	}
	provider := Detect(data)
	if provider == ProviderUnknown {
		return CompactionReport{Provider: provider, Compactions: []Compaction{}}, fmt.Errorf("unknown transcript format")
	}
	report := CompactionReport{Provider: provider, Compactions: []Compaction{}}
	switch provider {
	case ProviderClaude:
		report.Compactions = inspectClaudeCompactions(entries)
	case ProviderCodex:
		report.Compactions = inspectCodexCompactions(entries)
	}
	return report, nil
}

func inspectClaudeCompactions(entries []jsonLine) []Compaction {
	var boundaries []compactionCandidate
	var usages []usageCandidate
	for position, entry := range entries {
		obj := rawObject(entry.Raw)
		if entry.Type == "system" && stringValue(obj["subtype"]) == "compact_boundary" {
			metadata, _ := obj["compactMetadata"].(map[string]any)
			claude := parseClaudeCompaction(metadata)
			boundaries = append(boundaries, compactionCandidate{
				position: position,
				entry:    entry,
				refs:     []RawEventRef{rawEventRef(entry, position, obj)},
				claude:   claude,
			})
		}
		if entry.Type == "assistant" {
			if snapshot, ok := parseClaudeUsage(entry, position, obj); ok {
				usages = append(usages, usageCandidate{position: position, snapshot: snapshot})
			}
		}
	}
	return finishCompactions(boundaries, usages, ProviderClaude)
}

func inspectCodexCompactions(entries []jsonLine) []Compaction {
	var boundaries []compactionCandidate
	var contexts []compactionCandidate
	var usages []usageCandidate
	for position, entry := range entries {
		obj := rawObject(entry.Raw)
		if entry.Type == "compacted" {
			payload, _ := obj["payload"].(map[string]any)
			boundaries = append(boundaries, compactionCandidate{
				position: position,
				entry:    entry,
				refs:     []RawEventRef{rawEventRef(entry, position, obj)},
				codex:    parseCodexCompaction(payload),
			})
		}
		if entry.Type == "event_msg" {
			payload, _ := obj["payload"].(map[string]any)
			switch stringValue(payload["type"]) {
			case "context_compacted":
				contexts = append(contexts, compactionCandidate{
					position: position,
					entry:    entry,
					refs:     []RawEventRef{rawEventRef(entry, position, obj)},
				})
			case "token_count":
				if snapshot, ok := parseCodexUsage(entry, position, obj); ok {
					usages = append(usages, usageCandidate{position: position, snapshot: snapshot})
				}
			}
		}
	}

	if len(boundaries) == 0 {
		boundaries = contexts
	} else {
		usedContexts := map[int]bool{}
		for i := range boundaries {
			best := -1
			bestDistance := int(^uint(0) >> 1)
			for contextIndex, context := range contexts {
				if usedContexts[contextIndex] {
					continue
				}
				distance := context.position - boundaries[i].position
				if distance < 0 {
					distance = -distance
				}
				if distance < bestDistance {
					best = contextIndex
					bestDistance = distance
				}
			}
			if best >= 0 {
				usedContexts[best] = true
				boundaries[i].refs = append(boundaries[i].refs, contexts[best].refs...)
			}
		}
	}
	return finishCompactions(boundaries, usages, ProviderCodex)
}

func finishCompactions(boundaries []compactionCandidate, usages []usageCandidate, provider Provider) []Compaction {
	sort.SliceStable(boundaries, func(i, j int) bool { return boundaries[i].position < boundaries[j].position })
	out := make([]Compaction, 0, len(boundaries))
	for index, boundary := range boundaries {
		item := Compaction{
			Index:     index,
			Timestamp: boundary.entry.Timestamp,
			Provider:  provider,
			RawEvents: boundary.refs,
			Claude:    boundary.claude,
			Codex:     boundary.codex,
		}
		if boundary.claude != nil {
			item.DurationMs = boundary.claude.DurationMs
			item.PreTotalTokens = boundary.claude.PreTokens
			item.ProviderPostTokens = boundary.claude.PostTokens
			item.ProviderPostTokensSemantics = "payload_only"
			item.ProviderPostTokensSource = "system.compact_boundary.compactMetadata.postTokens"
		} else if boundary.codex != nil {
			item.ProviderPostTokensSemantics = "provider_reported_last_usage"
			item.ProviderPostTokensSource = "event_msg.token_count.info.last_token_usage.total_tokens"
		}

		before, after := nearestUsage(usages, boundary.position)
		if before != nil {
			item.NearestUsageBefore = &before.snapshot
		}
		if after != nil {
			item.NearestUsageAfter = &after.snapshot
			item.FirstFullUsageAfter = &after.snapshot
		}
		if item.Provider == ProviderClaude {
			if item.PreTotalTokens == nil && before != nil {
				item.PreTotalTokens = before.snapshot.ComputedTotalTokens
			}
			if item.ProviderPostTokens == nil && after != nil {
				item.ProviderPostTokens = after.snapshot.ComputedTotalTokens
			}
		} else if after != nil {
			item.ProviderPostTokens = after.snapshot.TotalTokens
			if item.ProviderPostTokens == nil {
				item.ProviderPostTokens = after.snapshot.ComputedTotalTokens
			}
			item.ProviderPostTokensSemantics = "provider_reported_last_usage"
		}
		if before != nil && item.PreTotalTokens == nil {
			item.PreTotalTokens = before.snapshot.TotalTokens
		}
		if after != nil {
			item.ContextWindow = after.snapshot.ModelContextWindow
		}
		if item.ContextWindow == nil && before != nil {
			item.ContextWindow = before.snapshot.ModelContextWindow
		}
		if item.ContextWindow != nil && item.ProviderPostTokens != nil {
			item.RemainingPercentage = remainingPercentage(*item.ContextWindow, *item.ProviderPostTokens)
		}
		out = append(out, item)
	}
	return out
}

func nearestUsage(usages []usageCandidate, position int) (*usageCandidate, *usageCandidate) {
	var before, after *usageCandidate
	for i := range usages {
		candidate := &usages[i]
		if candidate.position < position {
			before = candidate
			continue
		}
		if candidate.position > position {
			after = candidate
			break
		}
	}
	return before, after
}

func parseClaudeCompaction(metadata map[string]any) *ClaudeCompaction {
	if metadata == nil {
		return &ClaudeCompaction{}
	}
	return &ClaudeCompaction{
		Trigger:                 stringValue(metadata["trigger"]),
		PreTokens:               int64Pointer(metadata["preTokens"]),
		PostTokens:              int64Pointer(metadata["postTokens"]),
		DurationMs:              int64Pointer(metadata["durationMs"]),
		CumulativeDroppedTokens: int64Pointer(metadata["cumulativeDroppedTokens"]),
		PreservedSegment:        parseClaudePreservedSegment(metadata["preservedSegment"]),
		PreservedMessages:       parseClaudePreservedMessages(metadata["preservedMessages"]),
	}
}

func parseClaudePreservedSegment(value any) *ClaudePreservedSegment {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return &ClaudePreservedSegment{
		HeadUUID:   stringValue(m["headUuid"]),
		AnchorUUID: stringValue(m["anchorUuid"]),
		TailUUID:   stringValue(m["tailUuid"]),
	}
}

func parseClaudePreservedMessages(value any) *ClaudePreservedMessages {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	uuidStrings := func(value any) []string {
		items, _ := value.([]any)
		out := make([]string, 0, len(items))
		for _, item := range items {
			if uuid := stringValue(item); uuid != "" {
				out = append(out, uuid)
			}
		}
		return out
	}
	uuids := uuidStrings(m["uuids"])
	allUUIDs := uuidStrings(m["allUuids"])
	return &ClaudePreservedMessages{
		AnchorUUID: stringValue(m["anchorUuid"]),
		UUIDs:      uuids,
		AllUUIDs:   allUUIDs,
		Count:      len(uuids),
		AllCount:   len(allUUIDs),
	}
}

func parseCodexCompaction(payload map[string]any) *CodexCompaction {
	if payload == nil {
		return nil
	}
	history, _ := payload["replacement_history"].([]any)
	replacements := make([]CodexReplacement, 0, len(history))
	for _, value := range history {
		m, ok := value.(map[string]any)
		if !ok {
			continue
		}
		replacement := CodexReplacement{
			Type:                        stringValue(m["type"]),
			ID:                          stringValue(m["id"]),
			Role:                        stringValue(m["role"]),
			EncryptedContentPresent:     stringValue(m["encrypted_content"]) != "",
			InternalChatMessageMetadata: mapValue(m["internal_chat_message_metadata_passthrough"]),
		}
		if encrypted := stringValue(m["encrypted_content"]); encrypted != "" {
			replacement.EncryptedContentLength = len(encrypted)
		}
		if content, ok := m["content"].([]any); ok {
			replacement.ContentCount = len(content)
			for _, part := range content {
				if partMap, ok := part.(map[string]any); ok {
					if typ := stringValue(partMap["type"]); typ != "" {
						replacement.ContentTypes = append(replacement.ContentTypes, typ)
					}
				}
			}
		}
		replacements = append(replacements, replacement)
	}
	return &CodexCompaction{
		WindowID:                stringValue(payload["window_id"]),
		PreviousWindowID:        stringValue(payload["previous_window_id"]),
		FirstWindowID:           stringValue(payload["first_window_id"]),
		WindowNumber:            int64Pointer(payload["window_number"]),
		ReplacementHistory:      replacements,
		ReplacementHistoryCount: len(history),
	}
}

func parseClaudeUsage(entry jsonLine, position int, obj map[string]any) (UsageSnapshot, bool) {
	message, _ := obj["message"].(map[string]any)
	usage, _ := message["usage"].(map[string]any)
	if usage == nil {
		usage, _ = obj["usage"].(map[string]any)
	}
	if usage == nil {
		return UsageSnapshot{}, false
	}
	input := int64Pointer(usage["input_tokens"])
	output := int64Pointer(usage["output_tokens"])
	creation := int64Pointer(usage["cache_creation_input_tokens"])
	if creation == nil {
		creation = sumNestedUsage(usage["cache_creation"], "ephemeral_1h_input_tokens", "ephemeral_5m_input_tokens")
	}
	read := int64Pointer(usage["cache_read_input_tokens"])
	cacheWrite := int64Pointer(usage["cache_write_input_tokens"])
	computed := sumPointers(input, output, creation, read, cacheWrite)
	// Claude emits synthetic assistant records with an all-zero usage object
	// around a compaction boundary. They are lifecycle noise, not a full API
	// usage snapshot, so keep them out of nearest/first-full correlation.
	if computed == nil || *computed == 0 {
		return UsageSnapshot{}, false
	}
	snapshot := UsageSnapshot{
		Timestamp:                entry.Timestamp,
		Line:                     entry.Line,
		RecordIndex:              position,
		Provider:                 ProviderClaude,
		Source:                   "assistant.message.usage",
		RawType:                  entry.Type,
		MessageID:                stringValue(message["id"]),
		InputTokens:              input,
		OutputTokens:             output,
		CacheCreationInputTokens: creation,
		CacheReadInputTokens:     read,
		CacheWriteInputTokens:    cacheWrite,
		ComputedTotalTokens:      computed,
		ModelContextWindow:       firstInt64(usage["model_context_window"], usage["context_window"]),
	}
	if snapshot.ModelContextWindow != nil {
		snapshot.RemainingPercentage = remainingPercentage(*snapshot.ModelContextWindow, *computed)
	}
	return snapshot, true
}

func parseCodexUsage(entry jsonLine, position int, obj map[string]any) (UsageSnapshot, bool) {
	payload, _ := obj["payload"].(map[string]any)
	info, _ := payload["info"].(map[string]any)
	if info == nil {
		return UsageSnapshot{}, false
	}
	total := parseTokenUsageValues(info["total_token_usage"])
	last := parseTokenUsageValues(info["last_token_usage"])
	if total == nil && last == nil {
		return UsageSnapshot{}, false
	}
	active := last
	if active == nil {
		active = total
	}
	input := valueFromUsage(active, func(v *TokenUsageValues) *int64 { return v.InputTokens })
	output := valueFromUsage(active, func(v *TokenUsageValues) *int64 { return v.OutputTokens })
	computed := sumPointers(input, output)
	snapshot := UsageSnapshot{
		Timestamp:             entry.Timestamp,
		Line:                  entry.Line,
		RecordIndex:           position,
		Provider:              ProviderCodex,
		Source:                "event_msg.token_count.info",
		RawType:               entry.Type,
		PayloadType:           stringValue(payload["type"]),
		InputTokens:           input,
		OutputTokens:          output,
		CachedInputTokens:     valueFromUsage(active, func(v *TokenUsageValues) *int64 { return v.CachedInputTokens }),
		CacheWriteInputTokens: valueFromUsage(active, func(v *TokenUsageValues) *int64 { return v.CacheWriteInputTokens }),
		ReasoningOutputTokens: valueFromUsage(active, func(v *TokenUsageValues) *int64 { return v.ReasoningOutputTokens }),
		TotalTokens:           valueFromUsage(active, func(v *TokenUsageValues) *int64 { return v.TotalTokens }),
		ComputedTotalTokens:   computed,
		ModelContextWindow:    int64Pointer(info["model_context_window"]),
		TotalUsage:            total,
		LastUsage:             last,
	}
	if snapshot.ModelContextWindow != nil && snapshot.TotalTokens != nil {
		snapshot.RemainingPercentage = remainingPercentage(*snapshot.ModelContextWindow, *snapshot.TotalTokens)
	}
	return snapshot, true
}

func parseTokenUsageValues(value any) *TokenUsageValues {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	return &TokenUsageValues{
		InputTokens:           int64Pointer(m["input_tokens"]),
		CachedInputTokens:     int64Pointer(m["cached_input_tokens"]),
		CacheWriteInputTokens: int64Pointer(m["cache_write_input_tokens"]),
		OutputTokens:          int64Pointer(m["output_tokens"]),
		ReasoningOutputTokens: int64Pointer(m["reasoning_output_tokens"]),
		TotalTokens:           int64Pointer(m["total_tokens"]),
	}
}

func rawEventRef(entry jsonLine, position int, obj map[string]any) RawEventRef {
	payload, _ := obj["payload"].(map[string]any)
	return RawEventRef{
		Line:        entry.Line,
		RecordIndex: position,
		Timestamp:   entry.Timestamp,
		Type:        entry.Type,
		Subtype:     stringValue(obj["subtype"]),
		PayloadType: stringValue(payload["type"]),
		UUID:        stringValue(obj["uuid"]),
	}
}

func rawObject(raw json.RawMessage) map[string]any {
	var obj map[string]any
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&obj); err != nil {
		return nil
	}
	return obj
}

func mapValue(value any) map[string]any {
	m, _ := value.(map[string]any)
	return m
}

func sumNestedUsage(value any, keys ...string) *int64 {
	m, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	var values []*int64
	for _, key := range keys {
		values = append(values, int64Pointer(m[key]))
	}
	return sumPointers(values...)
}

func sumPointers(values ...*int64) *int64 {
	var total int64
	found := false
	for _, value := range values {
		if value != nil {
			total += *value
			found = true
		}
	}
	if !found {
		return nil
	}
	return &total
}

func firstInt64(values ...any) *int64 {
	for _, value := range values {
		if result := int64Pointer(value); result != nil {
			return result
		}
	}
	return nil
}

func int64Pointer(value any) *int64 {
	var result int64
	switch typed := value.(type) {
	case json.Number:
		parsed, err := typed.Int64()
		if err != nil {
			return nil
		}
		result = parsed
	case float64:
		result = int64(typed)
	case float32:
		result = int64(typed)
	case int:
		result = int64(typed)
	case int64:
		result = typed
	case string:
		parsed, err := json.Number(typed).Int64()
		if err != nil {
			return nil
		}
		result = parsed
	default:
		return nil
	}
	return &result
}

func valueFromUsage(value *TokenUsageValues, getter func(*TokenUsageValues) *int64) *int64 {
	if value == nil {
		return nil
	}
	return getter(value)
}

func remainingPercentage(contextWindow, used int64) *float64 {
	if contextWindow <= 0 {
		return nil
	}
	remaining := (1 - float64(used)/float64(contextWindow)) * 100
	return &remaining
}

// RenderCompactions writes the report as JSON or as a concise human-readable
// lifecycle summary. The text explicitly distinguishes Claude's compact
// payload size from the next full API usage snapshot.
func RenderCompactions(w io.Writer, report CompactionReport, format string) error {
	if strings.EqualFold(format, "json") {
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}
	if len(report.Compactions) == 0 {
		_, _ = fmt.Fprintf(w, "No compaction records found (%s).\n", report.Provider)
		return nil
	}
	_, _ = fmt.Fprintf(w, "%s compactions: %d\n\n", report.Provider, len(report.Compactions))
	for _, item := range report.Compactions {
		_, _ = fmt.Fprintf(w, "[%d] %s\n", item.Index+1, item.Timestamp)
		if item.Claude != nil {
			_, _ = fmt.Fprintf(w, "  Claude: trigger=%s pre=%s post=%s (payload-only) dropped=%s duration=%s\n", item.Claude.Trigger, formatInt64(item.Claude.PreTokens), formatInt64(item.Claude.PostTokens), formatInt64(item.Claude.CumulativeDroppedTokens), formatInt64(item.DurationMs))
		}
		if item.Codex != nil {
			_, _ = fmt.Fprintf(w, "  Codex: window=%s previous=%s number=%s replacements=%d\n", item.Codex.WindowID, item.Codex.PreviousWindowID, formatInt64(item.Codex.WindowNumber), item.Codex.ReplacementHistoryCount)
		}
		_, _ = fmt.Fprintf(w, "  normalized: pre_total_tokens=%s provider_post_tokens=%s context_window=%s remaining=%s%%\n", formatInt64(item.PreTotalTokens), formatInt64(item.ProviderPostTokens), formatInt64(item.ContextWindow), formatFloat(item.RemainingPercentage))
		if item.FirstFullUsageAfter != nil {
			usage := item.FirstFullUsageAfter
			_, _ = fmt.Fprintf(w, "  first full usage after: %s input=%s output=%s cache_creation=%s cache_read=%s computed_total=%s\n", usage.Timestamp, formatInt64(usage.InputTokens), formatInt64(usage.OutputTokens), formatInt64(usage.CacheCreationInputTokens), formatInt64(usage.CacheReadInputTokens), formatInt64(usage.ComputedTotalTokens))
		}
		if item.Provider == ProviderClaude {
			_, _ = fmt.Fprintln(w, "  note: Claude postTokens is the compact payload size; the first full usage may include reattached context and cache tokens")
		}
		_, _ = fmt.Fprintln(w)
	}
	return nil
}

func formatInt64(value *int64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *value)
}

func formatFloat(value *float64) string {
	if value == nil {
		return "-"
	}
	return fmt.Sprintf("%.2f", *value)
}
