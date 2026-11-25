---
title: Transcript model
description: Understand how agentscript normalizes Claude Code, Codex, and Antigravity CLI JSONL into stable renderable blocks and turns.
order: 3
category: Workflows
summary: How block indexes, turn indexes, filtering, folding, and slicing work.
---

## Stable blocks

agentscript parses Claude Code, Codex, and Antigravity CLI sessions into one stream of renderable blocks:

```text
#000 user
#001 thinking
#002 assistant
#003 command Bash
#004 command_result Bash
#005 tool_call Edit
#006 tool_result Edit
```

Indexes are assigned before filtering. This means `--hide-thinking` may show gaps, but the visible blocks still keep their original indexes.

Codex shell commands, edits, and tool calls inside code-mode `exec` have separate call and result blocks. Codex progress messages count as assistant messages. Image attachments appear as text markers, and tool results keep their index even when they have no text.

While a transcript is being written, an incomplete final JSON record is skipped. A valid final record is included even before its newline arrives.

Antigravity native logs contain user requests, model replies, reasoning, calls, results, images, and system messages. Commands retain the tool name `run_command`. The parser pairs results with calls in step order and represents attachments with text markers. Exported Antigravity ACP JSONL and headless JSON or stream-JSON are also readable. These exports can omit prompt text, reasoning, or tool output. For the richest view, open the native full log.

## Turns

A turn starts at each user message. Use `--turns` to display turn numbers:

```bash
agentscript open transcript.jsonl --turns
```

Then slice by turn:

```bash
agentscript open transcript.jsonl --turn-slice 4:7
```

To see recent activity, select the last numbered turns or blocks:

```bash
agentscript open transcript.jsonl --last-turns 3 --hide-tool-results
agentscript slice transcript.jsonl --last 80 --messages-only
```

Both tail flags work with `open` and `slice` and count before display filters. A turn includes the user message and the activity that follows it. The blocks before the first user message are outside numbered turns.

## Folding and expansion

Collapse long blocks with:

```bash
agentscript open transcript.jsonl --max-lines 40
```

Collapsed blocks include an exact expansion command:

```bash
agentscript open "transcript.jsonl" --around 120 --expand 120
```

Expand everything with:

```bash
agentscript open transcript.jsonl --max-lines 40 --expand all
```

## Output budget

Use `--max-chars` with `open` or `slice` to page the rendered transcript without splitting a block:

```bash
agentscript open transcript.jsonl --last-turns 3 --max-chars 20000
agentscript slice transcript.jsonl 100:300 --max-chars 20000 --format json
```

The continuation command starts at the first omitted block and keeps the original selection's end and rendering flags. In JSON, read the next stable block index from `next.index` and the command from `next.command`. The continuation metadata and minimum format framing can exceed the budget. If a page is empty because its first block is too large, its continuation raises the budget enough to fit that block.

## Profiles

Built-in profiles are shortcuts for common block sets:

```bash
agentscript open transcript.jsonl --profile compact
agentscript open transcript.jsonl --profile handoff
agentscript open transcript.jsonl --profile commands
agentscript open transcript.jsonl --profile tools
```

Custom profiles can be added to the config file shown by:

```bash
agentscript config path
```
