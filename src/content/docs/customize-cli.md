---
title: Transcript model
description: Understand how agentscript normalizes Claude Code and Codex JSONL into stable renderable blocks.
order: 3
category: Workflows
summary: How block indexes, filtering, and slicing work.
---

## Stable blocks

agentscript parses Claude Code and Codex sessions into one stream of renderable blocks:

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

## Why this matters

Stable indexes make long transcript surgery easy. For example, if a session changed tasks around block 100, preserve the earlier context with:

```bash
agentscript slice transcript.jsonl 0:100 --format md --out old-context.md
```

Then inspect the new task separately:

```bash
agentscript slice transcript.jsonl 100:
```

## Commands and tools

Shell commands are separated from generic tool calls so they can be hidden independently:

```bash
agentscript open transcript.jsonl --hide-commands
agentscript open transcript.jsonl --hide-tools
```

Tool and command results can be hidden while keeping the calls visible:

```bash
agentscript open transcript.jsonl --hide-tool-results
```
