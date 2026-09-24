---
title: Quickstart
description: Hand off, search, and inspect Claude Code and Codex JSONL transcripts.
order: 1
category: Start
summary: Install agentscript, transfer agent context, search history, and inspect transcripts.
---

## Install

```bash
npm i -g @amxv/agentscript
agentscript --help
```

For local development from a clone:

```bash
make build
./dist/agentscript --help
```

## Continue another agent's work

When a user gives you a transcript path and asks you to continue, start here:

```bash
agentscript handoff /path/to/transcript.jsonl
```

The output is designed for a receiving agent: latest substantive user requests (including context behind repeated `keep going` turns), a compact recent work trace, task-relevant file changes, Git/validation/failure activity, and drill-down commands. The live filesystem remains the source of truth.

These are equivalent shortcuts:

```bash
agentscript continue /path/to/transcript.jsonl
agentscript /path/to/transcript.jsonl
```

Full session UUIDs and `file://` transcript URLs are accepted too.

## Open a transcript

```bash
agentscript open ~/.claude/projects/<project>/<session>.jsonl
agentscript open ~/.codex/sessions/<year>/<month>/<day>/<session>.jsonl
```

Use `open` when you want raw/explicit transcript rendering rather than the continuation-focused handoff view. Running `agentscript` by itself shows help.

## Open a recent transcript

```bash
agentscript open --latest 1
agentscript list --latest 20
```

By default, agentscript discovers transcripts in:

```bash
~/.claude/projects
~/.codex/sessions
```

If `CLAUDE_CONFIG_DIR` or `CODEX_HOME` are set, agentscript uses their `projects` and `sessions` subdirectories while retaining the correct provider identity even when those paths are nonstandard.

Use a custom discovery root when needed:

```bash
agentscript list --roots ./fixtures,~/Desktop/transcripts
```

Filter recent sessions by project or age:

```bash
agentscript list --project agentscript --since 30d
agentscript open --project agentscript --latest 1
```

## Search history

Plain search is exhaustive across discovered history:

```bash
agentscript search "publish-pr"
agentscript search push rejected --near 20
agentscript search permission denied --any --provider codex
```

Results are grouped by session. The normalized search cache is incremental; inspect or prebuild it with:

```bash
agentscript index status
agentscript index rebuild --since 30d
```

## Hide noisy blocks

```bash
agentscript open transcript.jsonl --hide-thinking
agentscript open transcript.jsonl --messages-only
agentscript open transcript.jsonl --profile compact
```

## Slice and export context

```bash
agentscript slice transcript.jsonl 0:100 --format md --out context.md
agentscript export transcript.jsonl --format html --out transcript.html
```

## Inspect what happened

```bash
agentscript commands transcript.jsonl --failed --with-output
agentscript changes transcript.jsonl
agentscript activity transcript.jsonl
```
