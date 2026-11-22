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
make release-tag VERSION=0.4.0
```

That creates and pushes `v0.4.0`.

## What the workflow does

The GitHub Actions workflow:

1. runs Go and Node quality checks
2. builds binaries for all six targets and checks that `agentscript --version` reports the version from the tag on each platform
3. generates SHA-256 checksums and uploads the manifest with the binaries to a GitHub Release
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
agentscript_windows_arm64.exe
```

## Verify release binaries

Each release includes `checksums.txt` in standard `sha256sum` format, with one SHA-256 digest and filename per binary. Download the manifest and all six binaries into the same directory, then run one of these commands in that directory:

```bash
sha256sum -c checksums.txt
```

```bash
shasum -a 256 -c checksums.txt
```

## Required secret

Publishing to npm requires this GitHub secret:

```bash
NPM_TOKEN
```
