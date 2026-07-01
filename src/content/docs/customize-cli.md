---
title: Customize the CLI
description: Rename the starter command, update project identity, and replace the starter command handlers safely.
order: 2
category: Development
summary: The main checklist for turning mycli into a real Go command-line tool.
---

## Rename the command

The starter command is `mycli`. Rename it consistently before adding real behavior:

```bash
cmd/mycli                      # Go entrypoint directory
bin/mycli.js                   # npm executable shim
package.json                   # bin map and config.cliBinaryName
.github/workflows/release.yml  # CLI_BINARY
Makefile                       # BIN_NAME
```

The release workflow and postinstall script assume a single binary name. Keep those values in sync.

## Update package identity

Update package metadata before publishing:

```bash
package.json name
package.json description
package.json repository
package.json homepage
package.json bugs
package.json keywords
go.mod module path
```

The postinstall script reads repository/package metadata to find release binaries, so stale package metadata can break npm installs.

## Replace command logic

The starter app logic lives in:

```bash
internal/app/app.go
internal/app/app_test.go
```

Keep parsing and handlers small at first. Add command-specific help text as command groups grow.

## Keep version plumbing

`internal/buildinfo` exposes the version used by `--version`. Release builds inject the version with Go linker flags.

For local development, the Makefile reads the version from `package.json` and passes it into the build.

## Validate after changes

Run the full local check before pushing:

```bash
make check
bun run docs:check
bun run docs:build
```

Run the docs commands serially. Astro can be sensitive to concurrent check/build work in the same repo.
