# agentscript

`agentscript` is a terminal-first transcript reader for Claude Code and Codex JSONL sessions.

It turns local agent transcripts into readable, indexed blocks so you can search, inspect, hide noisy parts, slice exact context ranges, extract file and command activity, and export handoff-ready Markdown or HTML.

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

Basic search:

```bash
agentscript search "publish-pr"
agentscript search "r2 cors" --provider claude --latest 20
```

Advanced search:

```bash
agentscript search push rejected --all --near 20
agentscript search "git (push|pull)" --regex --tool Bash
agentscript search permission --search-kind command_result
agentscript search YOLO --case-sensitive
```

Each result includes the matching block index and an `agentscript open ... --around <index>` command.

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
