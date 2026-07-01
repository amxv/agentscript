# AGENTS.md

Guidance for coding agents working in `go-cli-template`.

## Purpose

This repo is a generic starter for Go command-line tools distributed through npm.

The sample command is `mycli`. Replace it with your actual CLI name and behavior.

## Architecture

- `cmd/mycli/main.go`: process entrypoint, error handling, exits non-zero on failure.
- `internal/app/app.go`: command parser + handlers.
- `internal/app/app_test.go`: starter tests.
- `bin/mycli.js`: npm shim that invokes packaged native binary.
- `scripts/postinstall.js`: downloads release binary on install, falls back to `go build`.
- `.github/workflows/release.yml`: tag-driven release pipeline.
- `src/`: ZueDocs-powered Astro documentation site for future CLIs.

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

## How to customize safely

1. Rename CLI command consistently in all places:
- directory `cmd/mycli`
- `package.json` values (`bin`, `config.cliBinaryName`)
- `bin/mycli.js`
- workflow env `CLI_BINARY`
- `Makefile` `BIN_NAME`

2. Keep binary naming convention unchanged unless you also update postinstall/workflow:
- release assets: `<cli>_<goos>_<goarch>[.exe]`
- npm-installed binary path: `bin/<cli>-bin` (or `.exe` on Windows)

3. If adding dependencies, commit `go.sum` and optionally enable Go cache in workflow.

4. Keep help output expressive and command-local (`<command> --help` should explain examples).

5. Update the bundled docs site when command behavior changes:
- `src/data/docs.ts` for site name, repo URL, nav, footer sections, and categories
- `src/pages/index.astro` for landing-page product copy
- `src/content/docs/*.md` for guides and command reference

6. ZueDocs consumer pattern:
- import shared layouts from `zuedocs/layouts/*`
- import shared behavior/styles from `zuedocs/docsEnhancements` and `zuedocs/styles.css`
- keep repo-specific content/config local instead of forking shared shell files

## Release contract

Release pipeline triggers on `v*` tags and expects:

- `NPM_TOKEN` GitHub secret present.
- npm package name in `package.json` is publishable under your account/org.
- repository URL matches the release origin used by `scripts/postinstall.js`.

## Guardrails

- Prefer additive changes; do not break the release asset naming contract unintentionally.
- If you change release artifacts or CLI binary name, update both workflow and postinstall script in the same PR.
- Run docs validation serially (`bun run docs:check` then `bun run docs:build`); do not run Astro check/build concurrently in the same repo.
