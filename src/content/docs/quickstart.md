---
title: Quickstart
description: Open Claude Code and Codex JSONL transcripts in a readable terminal format.
order: 1
category: Start
summary: Install agentscript, open a transcript, search sessions, slice context, and export output.
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

## Open a transcript

```bash
agentscript open ~/.claude/projects/<project>/<session>.jsonl
agentscript open ~/.codex/sessions/<year>/<month>/<day>/<session>.jsonl
```

`open` is the main entry point. Running `agentscript` by itself shows help instead of opening anything.

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

Use a custom discovery root when needed:

```bash
agentscript list --roots ./fixtures,~/Desktop/transcripts
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
