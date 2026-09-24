# agentscript

`agentscript` is a terminal-first handoff, search, and transcript reader for Claude Code and Codex JSONL sessions.

It makes agent-to-agent context transfer a one-command workflow, while also turning local transcripts into readable indexed blocks for search, inspection, slicing, file/command activity, and export.

## Install

```bash
npm i -g @amxv/agentscript
agentscript --help
```

For local development:

```bash
make build
./dist/agentscript --help
```

## Agent handoff — the primary workflow

When you give a new agent a transcript path and ask it to continue the work, the first command should be:

```bash
agentscript handoff /path/to/transcript.jsonl
```

`handoff` produces continuation-ready Markdown with the current task requests, a compact recent work trace, files changed during the task (or relevant earlier edits), Git/validation/failure activity, and exact drill-down commands. It deliberately treats the transcript as historical context and reminds the receiving agent to verify the live filesystem before making changes.

The natural alias and bare-path shortcut work too:

```bash
agentscript continue /path/to/transcript.jsonl
agentscript /path/to/transcript.jsonl
```

You can also use a full Claude/Codex session UUID instead of a path, or paste a `file://` URL:

```bash
agentscript handoff 019f91bc-123f-7692-8a78-21e54d6677e6
agentscript handoff 'file:///Users/me/.codex/sessions/.../rollout.jsonl'
```

Useful handoff controls:

```bash
agentscript handoff transcript.jsonl --last 80
agentscript handoff transcript.jsonl --format json
agentscript handoff transcript.jsonl --out HANDOFF.md
```

The default handoff repeats terse continuation turns such as `keep going` together with nearby substantive requests, hides thinking and bulky tool results, keeps stable block indexes, and limits command noise. Use the drill-down commands it prints when more history is needed.

## Open transcripts

Open a transcript by path:

```bash
agentscript open ~/.claude/projects/<project>/<session>.jsonl
agentscript open ~/.codex/sessions/<year>/<month>/<day>/<session>.jsonl
```

Open a Codex transcript by session ID:

```bash
agentscript codex 019f91bc-123f-7692-8a78-21e54d6677e6
```

Open a recent transcript:

```bash
agentscript open --latest 1
agentscript list --latest 20
```

Run `agentscript open` with no path to launch the latest-transcript picker. Discovery uses `$CLAUDE_CONFIG_DIR/projects` and `$CODEX_HOME/sessions` when those directories exist. Otherwise, it falls back to:

```bash
~/.claude/projects
~/.codex/sessions
```

Discovery keeps provider identity with the configured source, so custom `CLAUDE_CONFIG_DIR` and `CODEX_HOME` locations do not need `.claude` or `.codex` in their paths. A short-lived session catalog cache avoids repeatedly walking large transcript trees; use `--refresh` when you explicitly want to rescan them.

Filter discovery by project or age:

```bash
agentscript list --project agentscript --since 30d
agentscript open --project agentscript --latest 1
```

## Stable block indexes and turns

Every renderable block gets a stable index:

```text
#000 user
#001 thinking
#002 assistant
#003 command Bash
#004 command_result Bash
#005 tool_call Edit
#006 tool_result Edit
```

Hidden blocks keep their original indexes, so slices remain stable even when you hide thinking or tool output.

Show user-turn numbers too:

```bash
agentscript open transcript.jsonl --turns
```

Slice by turn:

```bash
agentscript open transcript.jsonl --turn-slice 2:4
```

## Filtering, profiles, and folding

```bash
agentscript open transcript.jsonl --hide-thinking
agentscript open transcript.jsonl --show-internal-goal
agentscript open transcript.jsonl --show-internal
agentscript open transcript.jsonl --messages-only
agentscript open transcript.jsonl --hide-tool-results
agentscript open transcript.jsonl --hide-commands
agentscript open transcript.jsonl --kind user,assistant,command
agentscript open transcript.jsonl --hide-kind thinking,tool_result
agentscript open transcript.jsonl --tools Bash,Edit
agentscript open transcript.jsonl --hide-tool Bash
```

Use built-in profiles:

```bash
agentscript open transcript.jsonl --profile compact
agentscript open transcript.jsonl --profile handoff
agentscript open transcript.jsonl --profile debug
agentscript open transcript.jsonl --profile commands
```

Collapse long blocks:

```bash
agentscript open transcript.jsonl --max-lines 40
```

Collapsed output includes an exact expand command:

```bash
agentscript open "transcript.jsonl" --around 120 --expand 120
```

Expand all collapsed blocks:

```bash
agentscript open transcript.jsonl --max-lines 40 --expand all
```

## Compaction and usage inspection

Inspect Claude and Codex lifecycle records without provider-specific `jq`:

```bash
agentscript compactions transcript.jsonl --format json
agentscript compactions transcript.jsonl
```

JSON output includes ordered boundaries, provider-native metadata, raw line references, nearest usage snapshots, and normalized token fields. Claude `postTokens` is labeled as payload-only; `first_full_usage_after` exposes the later assistant usage that may include reattached system, tool, and cache context.

## Slicing

Preserve the first 100 blocks of context:

```bash
agentscript slice transcript.jsonl 0:100 --format md --out context.md
```

Render from block 100 to the end:

```bash
agentscript slice transcript.jsonl 100:
```

Show context around a block:

```bash
agentscript open transcript.jsonl --around 100 --before 25 --after 50
```

Split a transcript into rendered files:

```bash
agentscript split transcript.jsonl --at 100 --out-dir parts --format md
agentscript split transcript.jsonl --every 80 --out-dir parts --format html
```

## Search

Search all discovered Claude Code and Codex history by default:

```bash
agentscript search "publish-pr"
agentscript search "r2 cors" --provider claude
agentscript search "old migration" --project agentscript --since 180d
```

Multiple unquoted query arguments are ANDed by default. Use `--any` for OR matching, or `--near` when the terms can occur in nearby blocks:

```bash
agentscript search push rejected
agentscript search permission denied --any
agentscript search push rejected --near 20
agentscript search "git (push|pull)" --regex --tool Bash
agentscript search permission --search-kind command_result
agentscript search YOLO --case-sensitive
```

Results are grouped by session and include matching stable block indexes plus both a ready-to-run `agentscript handoff ...` continuation command and an `agentscript open ... --around <index>` inspection command. `--limit` controls how many matching sessions are printed and `--hits` controls matches shown per session. `--latest N` is an explicit history filter rather than the default search scope.

Search uses an incremental normalized cache. Each indexed session gets a small metadata marker, a normalized text sidecar for fast candidate lookup, and a compressed structured block cache used to verify exact search semantics. Uncached history is candidate-filtered first and only matching transcripts are parsed. If `rg` is installed it is used as an accelerator, with a pure-Go fallback.

Inspect or prebuild the normalized cache:

```bash
agentscript index status
agentscript index rebuild --since 30d
agentscript index rebuild --project agentscript
agentscript index clear
```

The index is local and contains normalized transcript text; it is not encrypted separately from your filesystem. Use `AGENTSCRIPT_CACHE_DIR` to choose its location or `agentscript index clear` to remove it.

## Command, file, and activity views

Show shell commands:

```bash
agentscript commands transcript.jsonl
agentscript commands transcript.jsonl --failed --with-output
agentscript commands transcript.jsonl --grep "go test"
```

Show files referenced or changed:

```bash
agentscript files transcript.jsonl
agentscript changes transcript.jsonl
```

Summarize Git, PR, push, validation, and failure activity:

```bash
agentscript activity transcript.jsonl
agentscript activity transcript.jsonl --format json
```

`git` and `pr` are aliases for `activity`.

## Export

```bash
agentscript export transcript.jsonl --format html --out transcript.html
agentscript export transcript.jsonl --format md --md-style llm-context --slice 0:100 --out context.md
agentscript export transcript.jsonl --format json --out normalized.json
```

Markdown styles:

```bash
--md-style compact
--md-style llm-context
--md-style audit
```

## Config

Show the active config and built-in profiles:

```bash
agentscript config show
agentscript config path
```

Create a starter config:

```bash
agentscript config init
```

The config supports custom profiles with different block sets, hidden kinds, tool filters, max line folding, output format, and Markdown style.

## Development

```bash
make fmt
make test
make build
```

`tmp/claude-replay` is intentionally ignored and can be used as a local reference clone while working on parser behavior.
