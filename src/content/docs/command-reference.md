---
title: Command reference
description: Commands and flags for opening, listing, searching, filtering, slicing, and exporting transcripts.
order: 2
category: Reference
summary: The complete command map for agentscript.
---

## Root

```bash
agentscript --help
agentscript --version
```

## Open

```bash
agentscript open [path] [flags]
agentscript open --path <path> [flags]
agentscript open --latest 1 [flags]
```

Common flags:

```bash
--hide-thinking
--hide-tools
--hide-tool-results
--hide-commands
--messages-only
--tools Bash,Edit
--hide-tool Bash
--timestamps
--format text|md|json
--out <file>
```

## Slice

```bash
agentscript slice <path> 0:100
agentscript slice <path> 100:
agentscript slice <path> :50
```

Equivalent `open` flags:

```bash
agentscript open transcript.jsonl --slice 0:100
agentscript open transcript.jsonl --from 0 --to 100
agentscript open transcript.jsonl --last 80
agentscript open transcript.jsonl --around 100 --before 25 --after 50
```

## Search

```bash
agentscript search "publish-pr"
agentscript search "r2 cors" --provider claude --latest 20
agentscript search "git status" --provider codex
```

Each match includes the stable block index and an `open --around` command.

## List

```bash
agentscript list --latest 50
agentscript list --provider claude
agentscript list --provider codex
```
