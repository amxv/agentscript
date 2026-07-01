# agentscript

`agentscript` is a terminal-first transcript reader for Claude Code and Codex JSONL sessions.

It turns local agent transcripts into readable, indexed blocks so you can search, inspect, hide noisy parts, and slice exactly the context you want to preserve.

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

## Core workflow

Open a transcript by path:

```bash
agentscript open ~/.claude/projects/<project>/<session>.jsonl
agentscript open ~/.codex/sessions/<year>/<month>/<day>/<session>.jsonl
```

Or open the latest transcript discovered under the default roots:

```bash
agentscript open --latest 1
```

Run `agentscript open` with no path to launch the latest-transcript picker. The picker searches:

```bash
~/.claude/projects
~/.codex/sessions
```

## Readable block indexes

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

## Slicing

Preserve the first 100 blocks of context:

```bash
agentscript slice transcript.jsonl 0:100 --format md --out context.md
```

Render from block 100 to the end:

```bash
agentscript slice transcript.jsonl 100:
```

Show the last 80 renderable blocks:

```bash
agentscript open transcript.jsonl --last 80
```

Show context around a specific block:

```bash
agentscript open transcript.jsonl --around 100 --before 25 --after 50
```

## Filtering and toggles

```bash
agentscript open transcript.jsonl --hide-thinking
agentscript open transcript.jsonl --hide-tools
agentscript open transcript.jsonl --hide-tool-results
agentscript open transcript.jsonl --hide-commands
agentscript open transcript.jsonl --messages-only
agentscript open transcript.jsonl --hide-tool Bash
agentscript open transcript.jsonl --tools Bash,Edit
```

Flags can appear before or after the transcript path.

## Search

Search recent transcripts:

```bash
agentscript search "publish-pr"
agentscript search "r2 cors" --provider claude --latest 20
agentscript search "git status" --provider codex
```

Each result includes the matching block index and an `agentscript open ... --around <index>` command.

## List discovered transcripts

```bash
agentscript list --latest 20
agentscript list --provider codex
```

## Output formats

```bash
agentscript open transcript.jsonl --format text
agentscript open transcript.jsonl --format md --out transcript.md
agentscript open transcript.jsonl --format json --out normalized.json
```

## Development

```bash
make fmt
make test
make build
```

`tmp/claude-replay` is intentionally ignored and can be used as a local reference clone while working on parser behavior.
