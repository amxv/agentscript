---
title: Release and npm publishing
description: Understand the tag-driven GitHub release workflow and the npm wrapper package contract.
order: 3
category: Distribution
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

The npm package itself stays small. It contains the JavaScript shim, postinstall script, Go source, and project files needed for fallback local builds.

## Binary asset names

Release assets use this shape:

```bash
<cli>_<goos>_<goarch>[.exe]
```

Examples:

```bash
mycli_darwin_arm64
mycli_linux_amd64
mycli_windows_amd64.exe
```

Do not change this naming convention unless you update the workflow and `scripts/postinstall.js` together.

## npm install behavior

After npm install, `scripts/postinstall.js` tries to download the right native binary from GitHub Releases.

If the download cannot be completed, it falls back to building locally with Go when possible.

## Required secret

Publishing to npm requires this GitHub secret:

```bash
NPM_TOKEN
```

Make sure the package name in `package.json` is available and publishable before tagging a release.
