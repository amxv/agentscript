---
title: Command reference
description: Commands and flags for handoff, search, opening, filtering, slicing, export, and transcript inspection.
order: 2
category: Reference
summary: The complete command map for agentscript.
---

## Root

```bash
agentscript --help
agentscript --version
```

## Handoff

`handoff` is the agent-to-agent continuation command. A receiving agent should use it first when given a transcript path and asked to continue prior work.

```bash
agentscript handoff <path-or-session-id>
agentscript continue <path-or-session-id>
agentscript <path.jsonl>
```

It emits Markdown by default and includes current-task requests, recent work, changed files, task-scoped activity, and drill-down commands. Copied `file://` URLs and quoted `~` paths are normalized automatically.

Flags:

```bash
--last 40
--user-turns 3
--max-lines 30
--files 20
--commands 10
--format md|json
--out <file>
--roots <paths>
```

`--last` counts visible handoff blocks after noisy thinking/results are removed. Command breadcrumbs are capped separately by `--commands`.

## Open

```bash
agentscript open [path] [flags]
agentscript open --path <path> [flags]
agentscript open --latest 1 [flags]
```

On Windows, use drive paths, UNC paths, and extended paths such as `\\?\C:\logs\session.jsonl`. Use `file:///C:/logs/session.jsonl` for a drive or `file://server/share/session.jsonl` for a share. The same input syntax works for other commands that take transcript paths.

Use `agentscript antigravity <uuid>` (alias `agy`) to open an Antigravity CLI conversation by ID. Add `--roots <brain-directory>` for a custom profile. The provider flag is `--provider antigravity`.

Discovery filters:

```bash
--provider claude|codex|antigravity
--project <name-or-cwd-fragment>
--since 30d|2w|2026-09-01|<RFC3339>
--refresh
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
--max-chars 20000
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
agentscript slice <path> --last 80
agentscript slice <path> --last-turns 3
agentscript slice <path> --turn-slice 2:4
```

Equivalent `open` flags:

```bash
agentscript open transcript.jsonl --slice 0:100
agentscript open transcript.jsonl --from 0 --to 100
agentscript open transcript.jsonl --last 80
agentscript open transcript.jsonl --last-turns 3
agentscript open transcript.jsonl --around 100 --before 25 --after 50
```

Turn slicing:

```bash
agentscript open transcript.jsonl --turns --turn-slice 2:4
```

`--last N` counts blocks and `--last-turns N` counts numbered user turns. Both select before display filters and keep stable indexes. Use a tail flag in place of the range with `slice`. Use `--last-turns` on its own as the selection with `open`.

Page the output of either command:

```bash
agentscript open transcript.jsonl --last-turns 3 --max-chars 20000 --format json
agentscript slice transcript.jsonl 100:300 --max-chars 20000 --hide-thinking
```

`--max-chars N` limits rendered characters at block boundaries. The default, `0`, is unlimited. When a block is omitted, the output includes an `agentscript open --path … --slice START:END --max-chars N …` command. It resumes at the omitted block, keeps the original end index and rendering flags, and writes to stdout. JSON emits a `next` object with `index` and `command` only when more blocks remain. The budget excludes continuation metadata. Minimum format framing is always emitted, and a continuation on an empty page raises the budget when necessary to fit the first omitted block.

## Search

```bash
agentscript search "publish-pr"
agentscript search "r2 cors" --provider claude
agentscript search push rejected
agentscript search push rejected --near 20
agentscript search permission denied --any
agentscript search "git (push|pull)" --regex --tool Bash
agentscript search permission --search-kind command_result
```

Search covers all discovered history unless `--latest N` is supplied. Multiple query arguments are ANDed by default. Results are grouped by session.

Text results print a `continue:` handoff command for each matching session, followed by an `inspect:` command around the matched block.

Search flags:

```bash
--regex
--case-sensitive
--any
--near <blocks>
--search-kind <kinds>
--tool <names>
--provider claude|codex|antigravity
--project <name-or-cwd-fragment>
--since <date-or-duration>
--latest <sessions>
--limit <matching-sessions>
--hits <hits-per-session>
--format json
--out <file>
--refresh
```

`--all` remains accepted as a compatibility alias; AND matching is now the default.

## Index

The normalized search cache is incremental. Indexed sessions use a metadata marker, normalized text sidecar, and compressed structured block cache. Candidate lookup runs against the normalized text first; only matching structured entries are decoded. Uncached transcripts are candidate-filtered before parsing.

```bash
agentscript index status
agentscript index status --format json
agentscript index rebuild
agentscript index rebuild --project agentscript --since 30d
agentscript index clear
```

Set `AGENTSCRIPT_CACHE_DIR` to override the platform cache directory.

The cache contains normalized transcript text and is local to your machine; it is not separately encrypted. `agentscript index clear` removes both current and legacy search-index data.

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

## Compactions

```bash
agentscript compactions transcript.jsonl
agentscript compactions transcript.jsonl --format json
```

Antigravity stores completed compactions as checkpoint steps. The report links each checkpoint to its raw record and includes surrounding request usage when available. ACP captures only report explicitly recorded compaction events.

The command auto-detects Claude Code, Codex, and Antigravity CLI transcripts. JSON output includes ordered compaction boundaries, provider-native window or preserved-message metadata, raw line references, nearest usage snapshots, and normalized pre/post token fields. Claude `postTokens` is explicitly payload-only, while `first_full_usage_after` reports the next complete assistant usage snapshot.

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
