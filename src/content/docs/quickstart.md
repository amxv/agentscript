---
title: Quickstart
description: Install dependencies, run the starter CLI, and start the bundled ZueDocs site.
order: 1
category: Start
summary: The fastest path from a fresh clone to a working CLI and local docs site.
---

## Clone and install

Install Go, Node.js, and Bun, then install JavaScript dependencies:

```bash
bun install
```

The repository uses Go for the CLI, Node for the npm wrapper, and Astro/ZueDocs for the docs site.

## Run the starter CLI

Use the make targets to validate and build the starter command:

```bash
make check
make build
./dist/mycli --help
./dist/mycli hello
```

The sample command is intentionally small so it is easy to replace.

## Start the docs site

Run the embedded documentation site locally:

```bash
bun run docs:dev
```

Astro usually serves the site at `http://localhost:4321`.

## First customization pass

For a new CLI, start by renaming the command everywhere:

```bash
cmd/mycli
bin/mycli.js
package.json
.github/workflows/release.yml
Makefile
src/data/docs.ts
```

Then replace the starter command behavior in:

```bash
internal/app/app.go
internal/app/app_test.go
```

Keep the docs open while you edit so the quickstart, command reference, and release notes stay aligned with the actual CLI.
