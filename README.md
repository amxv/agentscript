# go-cli-template

Minimal template for shipping a Go CLI with:

- a local command runner (`Makefile`)
- npm global install wrapper (`bin/mycli.js`)
- automatic GitHub Release + npm publish on tag
- bundled ZueDocs-powered docs site

## Install (template example)

```bash
npm i -g @amxv/go-cli-template
mycli --help
```

## Commands in this starter

```bash
mycli --help
mycli hello
mycli hello <name>
mycli --version
```

## Docs site

This template includes an Astro docs site powered by ZueDocs.

```bash
bun install
bun run docs:dev
bun run docs:check
bun run docs:build
```

Customize the docs alongside the CLI:

- `src/data/docs.ts`: site name, repo URL, footer sections, nav, categories
- `src/pages/index.astro`: landing page
- `src/content/docs/*.md`: guides and command reference

## Customize this template

1. Rename your command and entrypoint:
- `cmd/mycli`
- `bin/mycli.js`
- `package.json` (`bin`, `config.cliBinaryName`)
- `.github/workflows/release.yml` (`CLI_BINARY`)

2. Update module + repo identity:
- `go.mod` module path
- `package.json` (`name`, `repository`, `homepage`, `bugs`)

3. Replace starter logic:
- `internal/app/app.go`
- `internal/app/app_test.go`

4. Update bundled docs:
- `src/data/docs.ts`
- `src/pages/index.astro`
- `src/content/docs/*.md`

5. Keep release flow:
- push tags like `v0.2.0`
- workflow builds binaries + creates GitHub release + publishes npm

## Project layout

- `cmd/mycli/main.go`: CLI entrypoint
- `internal/app/`: command logic
- `internal/buildinfo/`: build-time version plumbing for `--version`
- `scripts/postinstall.js`: installs binary from GitHub release (falls back to local `go build`)
- `.github/workflows/release.yml`: automated release pipeline
- `src/`: ZueDocs-powered documentation site
- `astro.config.mjs`: docs site build config
- `AGENTS.md`: instructions for coding agents
- `CONTRIBUTORS.md`: maintainer/release operations

See `AGENTS.md` and `CONTRIBUTORS.md` for complete dev/release instructions.

## License

Apache 2.0
