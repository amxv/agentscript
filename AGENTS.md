# AGENTS.md

Guidance for coding agents working in `agentscript`.

## Purpose

`agentscript` is a Go CLI for reading Claude Code and Codex JSONL transcripts. It normalizes both formats into stable renderable blocks so users can search, filter, and slice transcript ranges from the terminal.

## Architecture

- `cmd/agentscript/main.go`: process entrypoint, error handling, exits non-zero on failure.
- `internal/app/app.go`: command parser, CLI flags, picker, command handlers.
- `internal/transcript/`: format detection, Claude/Codex parsers, rendering, discovery, search, slicing.
- `internal/app/app_test.go`: command-level tests.
- `bin/agentscript.js`: npm shim that invokes packaged native binary.
- `scripts/postinstall.js`: downloads release binary on install, falls back to `go build`.
- `.github/workflows/release.yml`: tag-driven release pipeline.
- `src/`: ZueDocs-powered Astro documentation site.

## Local commands

Use `make` targets:

- `make fmt`
- `make test`
- `make vet`
- `make lint`
- `make check`
- `make build`
- `make build-all`
- `make install-local`

Direct commands:

- `go test ./...`
- `go vet ./...`
- `npm run lint`
- `bun run docs:check`
- `bun run docs:build`

## Parser notes

- Keep every renderable block indexed independently. User messages, assistant messages, thinking, tool calls, command calls, and result blocks should all preserve stable `#000`-style indexes.
- Filtering must not renumber blocks. Hidden thinking/tool output should create gaps, not new indexes.
- `claude-replay` can be cloned into `tmp/claude-replay` as a parser reference; `tmp/` is ignored and should not be committed.
- Prefer parser tests with small JSONL fixtures when changing format behavior.

## CLI design guardrails

- `agentscript` without a command should show help only. `open` is the main entry point.
- Non-interactive usage is first-class: every TUI/picker behavior should have flag equivalents.
- Flags should work before or after positional paths/ranges.
- Keep command-local help expressive and example-heavy.

## Release contract

Release pipeline triggers on `v*` tags and expects:

- `NPM_TOKEN` GitHub secret present.
- npm package name in `package.json` is publishable under your account/org.
- release assets named `<cli>_<goos>_<goarch>[.exe]`.
