---
title: Transcript model
description: Understand how agentscript normalizes Claude Code and Codex JSONL into stable renderable blocks and turns.
order: 3
category: Workflows
summary: How block indexes, turn indexes, filtering, folding, and slicing work.
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

## Turns

A turn starts at each user message. Use `--turns` to display turn numbers:

```bash
agentscript open transcript.jsonl --turns
```

Then slice by turn:

```bash
agentscript open transcript.jsonl --turn-slice 4:7
```

Block indexes remain the primary addressing system; turn indexes are a convenience for conversation-level ranges.

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
