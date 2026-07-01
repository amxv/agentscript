---
title: Release and npm publishing
description: Understand the tag-driven GitHub release workflow and npm wrapper package contract.
order: 5
category: Reference
summary: How native binaries, GitHub releases, and npm publishing fit together.
---

## Release trigger

The release workflow runs when a `v*` tag is pushed:

```bash
make release-tag VERSION=0.2.0
```

That creates and pushes `v0.2.0`.

## What the workflow does

The GitHub Actions workflow:

1. runs Go and Node quality checks
2. builds native binaries for supported OS/architecture targets
3. uploads binaries to a GitHub Release
4. publishes the npm package with the version from the tag

## Binary asset names

Release assets use this shape:

```bash
agentscript_<goos>_<goarch>[.exe]
```

Examples:

```bash
agentscript_darwin_arm64
agentscript_linux_amd64
agentscript_windows_amd64.exe
```

## Required secret

Publishing to npm requires this GitHub secret:

```bash
NPM_TOKEN
```
