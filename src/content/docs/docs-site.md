---
title: Development
description: Build, test, and maintain the agentscript repository.
order: 4
category: Reference
summary: Local development commands and repository notes.
---

## Go commands

```bash
make fmt
make test
make vet
make build
make check
```

## Docs commands

```bash
bun install
bun run docs:dev
bun run docs:check
bun run docs:build
```

## Parser reference

When working on parser behavior, clone claude-replay into the ignored local `tmp/` folder:

```bash
mkdir -p tmp
git clone https://github.com/es617/claude-replay.git tmp/claude-replay
```

The Go parser and transcript features live in:

```bash
internal/transcript/parser.go      # Claude/Codex parsers
internal/transcript/render.go      # text, Markdown, JSON, HTML rendering + folding
internal/transcript/search.go      # advanced search
internal/transcript/analysis.go    # commands, files, changes, activity
internal/transcript/config.go      # built-in and user profiles
internal/transcript/slice.go       # block and turn slicing
```

## CLI handlers

Command parsing lives in:

```bash
internal/app/app.go
internal/app/extra.go
```

Keep non-interactive flags first-class. Any TUI or picker behavior should have an equivalent flag-based workflow.
