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

The Go parser code lives in:

```bash
internal/transcript/parser.go
internal/transcript/render.go
internal/transcript/search.go
internal/transcript/slice.go
```
