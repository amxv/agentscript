package transcript

import "sort"

type AntigravityCompaction struct {
	Event     string `json:"event"`
	StepIndex *int64 `json:"step_index,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// The native step index orders parallel completions. The raw event references
// always point to the physical record in the original file.
func inspectAntigravityCompactions(entries []jsonLine) []Compaction {
	var boundaries []compactionCandidate
	var usages []usageCandidate
	for i, entry := range entries {
		obj := rawObject(entry.Raw)
		step := obj
		kind := entry.Type
		state := stringValue(step["status"])
		usage := obj
		source := "PLANNER_RESPONSE"
		entry.Timestamp = stringValue(obj["created_at"])
		switch obj["event"] {
		case "step_update":
			step = mapValue(obj["step_update"])
			kind = stringValue(step["step_type"])
			state = stringValue(step["state"])
			usage = mapValue(step["usage"])
			source = "step_update.usage"
		}
		index := int64Pointer(step["step_index"])
		position := i
		if index != nil {
			position = int(*index)
		}
		if (kind == "CHECKPOINT" || kind == "checkpoint") && state == "DONE" {
			entry.Type = kind
			boundaries = append(boundaries, compactionCandidate{
				position: position, entry: entry,
				refs:        []RawEventRef{rawEventRef(entry, i, obj)},
				antigravity: &AntigravityCompaction{Event: kind, StepIndex: index, Summary: stringValue(step["content"])},
			})
		}
		if kind == "PLANNER_RESPONSE" || kind == "agent_response" && state == "DONE" {
			input, read, output := int64Pointer(usage["input_tokens"]), int64Pointer(usage["cache_read_tokens"]), int64Pointer(usage["output_tokens"])
			if input != nil || output != nil {
				total := int64Pointer(usage["total_tokens"])
				if total == nil {
					total = sumPointers(input, read, output)
				}
				usages = append(usages, usageCandidate{position: position, snapshot: UsageSnapshot{
					Timestamp: entry.Timestamp, Line: entry.Line, RecordIndex: i,
					Provider: ProviderAntigravity, RawType: kind, Source: source,
					InputTokens: input, OutputTokens: output, CacheReadInputTokens: read,
					TotalTokens: total, ComputedTotalTokens: total,
				}})
			}
		}
		params := mapValue(mapValue(obj["msg"])["params"])
		update := mapValue(params["update"])
		if update["sessionUpdate"] == "compaction_update" && update["status"] == "completed" {
			entry.Type = "compaction_update"
			boundaries = append(boundaries, compactionCandidate{position: i, entry: entry, refs: []RawEventRef{rawEventRef(entry, i, obj)}, antigravity: &AntigravityCompaction{Event: entry.Type}})
		}
	}
	sort.SliceStable(usages, func(i, j int) bool { return usages[i].position < usages[j].position })
	return finishCompactions(boundaries, usages, ProviderAntigravity)
}
