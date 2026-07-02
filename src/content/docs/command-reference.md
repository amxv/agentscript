---
title: Command reference
description: Commands and flags for opening, listing, searching, filtering, slicing, splitting, exporting, and inspecting transcripts.
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
--profile full|compact|messages|handoff|debug|commands|tools
--hide-thinking
--show-thinking
--hide-tools
--show-tools
--hide-tool-results
--show-tool-results
--hide-commands
--show-commands
--messages-only
--kind user,assistant,command
--hide-kind thinking,tool_result
--tools Bash,Edit
--hide-tool Bash
--timestamps
--turns
--max-lines 40
--expand 120
--expand all
--format text|md|html|json
--md-style compact|llm-context|audit
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

Turn slicing:

```bash
agentscript open transcript.jsonl --turns --turn-slice 2:4
```

## Search

```bash
agentscript search "publish-pr"
agentscript search "r2 cors" --provider claude --latest 20
agentscript search push rejected --all --near 20
agentscript search "git (push|pull)" --regex --tool Bash
agentscript search permission --search-kind command_result
```

Search flags:

```bash
--regex
--case-sensitive
--all
--near <blocks>
--search-kind <kinds>
--tool <names>
```

## Commands

```bash
agentscript commands transcript.jsonl
agentscript commands transcript.jsonl --failed
agentscript commands transcript.jsonl --with-output
agentscript commands transcript.jsonl --grep "go test"
```

## Files and changes

```bash
agentscript files transcript.jsonl
agentscript changes transcript.jsonl
agentscript files transcript.jsonl --format json
```

## Activity

```bash
agentscript activity transcript.jsonl
agentscript git transcript.jsonl
agentscript pr transcript.jsonl
```

These commands summarize branches, commits, PR URLs, pushes, validation commands, and failures.

## Export

```bash
agentscript export transcript.jsonl --format html --out transcript.html
agentscript export transcript.jsonl --format md --md-style llm-context --slice 0:100 --out context.md
agentscript export transcript.jsonl --format json --out normalized.json
```

## Split

```bash
agentscript split transcript.jsonl --at 100 --out-dir parts --format md
agentscript split transcript.jsonl --every 80 --out-dir parts --format html
```

## Config

```bash
agentscript config show
agentscript config path
agentscript config init
```
