package transcript

import (
	"path"
	"strings"
)

// Codex emits completed activity items for commands and tools called inside
// code-mode exec. Those calls have their own IDs and do not appear as separate
// response items. Message and reasoning records are read from response items.
func codexCompletedToolBlocks(entry jsonLine, item map[string]any) []Block {
	id := stringValue(item["id"])
	var name, output string
	var input map[string]any
	kind := KindToolCall
	failed := codexItemFailed(item)
	switch stringValue(item["type"]) {
	case "CommandExecution":
		kind, name = KindCommand, "Bash"
		command := codexCommandText(item["command"])
		if cwd := stringValue(item["cwd"]); cwd != "" {
			command = "cd " + shellArgument(cwd) + " && " + command
		}
		input = map[string]any{"command": command}
		output = stringValue(item["aggregated_output"])
	case "FileChange":
		name = "Edit"
		input = map[string]any{"changes": item["changes"]}
		output = strings.TrimSpace(stringValue(item["stdout"]))
		if stderr := strings.TrimSpace(stringValue(item["stderr"])); stderr != "" {
			output = strings.TrimSpace(output + "\n" + stderr)
		}
	case "DynamicToolCall":
		name = stringValue(item["tool"])
		input = codexActivityArguments(item["arguments"])
		items, _ := item["content_items"].([]any)
		output = codexResultContent(items)
		success, _ := item["success"].(bool)
		failed = failed || (item["success"] != nil && !success)
	case "McpToolCall":
		name = stringValue(item["tool"])
		if server := stringValue(item["server"]); server != "" {
			name = "mcp__" + server + "__" + name
		}
		input = codexActivityArguments(item["arguments"])
		result := mapValue(item["result"])
		items, _ := result["content"].([]any)
		output = codexResultContent(items)
		if structured := stringValue(result["structuredContent"]); structured != "" {
			output = strings.TrimSpace(output + "\n" + structured)
		}
		isError, _ := result["isError"].(bool)
		failed = failed || isError
	default:
		return nil
	}
	return []Block{
		{Kind: kind, Role: "assistant", ToolID: id, ToolName: name, ToolInput: input, Timestamp: entry.Timestamp, Raw: entry.Raw},
		resultBlockFor(id, pendingTool{Name: name, Kind: kind}, output, entry.Timestamp, failed, entry.Raw),
	}
}

// Prefer the original call and result when the provider emits both transports
// for the same call ID. A completed item can still supply a missing half.
func mergeCodexToolActivity(blocks []Block, events []int, calls map[string]pendingTool, results map[string]bool) []Block {
	if len(events) == 0 {
		return blocks
	}
	out := blocks[:0]
	start := 0
	for _, pos := range events {
		out = append(out, blocks[start:pos]...)
		call, result := blocks[pos], blocks[pos+1]
		direct, hasCall := calls[call.ToolID]
		if !hasCall {
			out = append(out, call)
		}
		if !results[call.ToolID] {
			if hasCall {
				result = resultBlockFor(result.ToolID, direct, result.Text, result.Timestamp, result.IsError, result.Raw)
			}
			out = append(out, result)
		}
		start = pos + 2
	}
	return append(out, blocks[start:]...)
}

func codexActivityArguments(value any) map[string]any {
	if input := mapValue(value); input != nil {
		return input
	}
	if value != nil {
		return map[string]any{"raw": value}
	}
	return nil
}

func codexItemFailed(item map[string]any) bool {
	return stringValue(item["status"]) == "failed" || numberValue(item["exit_code"]) != 0
}

func codexResultContent(items []any) string {
	var parts []string
	for _, item := range items {
		var text string
		if block, ok := item.(map[string]any); ok {
			switch stringValue(block["type"]) {
			case "image", "inputImage", "input_image", "output_image":
				text = imageAttachment
			default:
				text = stringValue(block["text"])
				if text == "" {
					if _, hasText := block["text"]; !hasText {
						text = stringValue(block)
					}
				}
			}
		} else {
			text = stringValue(item)
		}
		parts = append(parts, text)
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

func codexCommandText(command any) string {
	argv, ok := command.([]any)
	if !ok {
		return cleanShellCommand(stringValue(command))
	}
	if len(argv) == 3 {
		switch path.Base(stringValue(argv[0])) {
		case "sh", "bash", "zsh":
			switch argv[1] {
			case "-c", "-lc":
				return stringValue(argv[2])
			}
		}
	}
	args := make([]string, 0, len(argv))
	for _, arg := range argv {
		args = append(args, shellArgument(stringValue(arg)))
	}
	return strings.Join(args, " ")
}

func shellArgument(arg string) string {
	const safe = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-._/:+=,@%"
	if arg != "" && strings.Trim(arg, safe) == "" {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\"'\"'") + "'"
}
