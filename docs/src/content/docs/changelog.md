---
title: Changelog
description: Release history for the agentscript CLI.
order: 6
category: Reference
summary: What changed in each agentscript release.
---

## v0.5.0 (2026-11-25)

- Added Antigravity CLI support (`--provider antigravity`) with parity across `open`, `slice`, `handoff`, `search`, `list`, session ID lookup, `compactions`, and change and command activity.
- Parses native Antigravity `transcript.jsonl` logs, ACP captures, and headless stream and JSON exports into the same stably indexed blocks as Claude Code and Codex.
- Discovery finds Antigravity transcripts under `~/.gemini/antigravity-cli/brain/<uuid>/.system_generated/logs/` on macOS and Linux, and `%USERPROFILE%\.gemini\antigravity-cli\brain\<uuid>\.system_generated\logs\` on Windows. Use `--roots` for custom locations.

## v0.4.0 (2026-11-22)

- `open` and `slice` accept `--last-turns N` to show only the last N user-started turns. `slice` also accepts `--last N` in place of a positional range. Stable block indexes are preserved.
- `--max-chars N` on `open` and `slice` caps total rendered output at block boundaries and prints the exact command that continues from the next block. JSON output includes `next.index` and `next.command` when truncated.
- Transcripts that are still being written are readable: an unterminated partial last line is skipped instead of failing.
- Transcript references accept Windows drive, UNC, and `\\?\` paths, plus `file://` URLs for drive and UNC paths.
- Releases now publish `checksums.txt` (SHA-256, `sha256sum` format) alongside the six platform binaries, and every binary's `--version` is verified on its native runner.
- GitHub Actions workflows run on Node 24 action runtimes.

## v0.3.2 (2026-11-22)

Compatibility update for current Claude Code (2.1.32x) and Codex (0.171) transcripts.

- Codex: `open` now renders shell commands, file changes, dynamic tools, and MCP calls and results recorded as completed code-mode activity.
- Codex: structured tool output renders as text, standalone results keep their tool names and call IDs, and commentary appears as assistant messages.
- Claude Code: image and tool-reference results are now visible.
- Empty tool and command results keep their indexed headers, so block numbering stays stable.
- Discovery: switching provider filters works correctly with custom `CLAUDE_CONFIG_DIR` and `CODEX_HOME` roots.
- Performance: Claude transcripts are parsed record by record, reducing time and peak memory on large sessions.

## v0.3.1 (2026-09-25)

- Release binaries now embed their version, so `agentscript --version` reports the published release.

## v0.3.0 (2026-09-25)

- Added `handoff`, the primary agent-to-agent transfer workflow. A bare `.jsonl` or `file://` reference behaves like `handoff`.
- Added `search` across Claude Code and Codex history, with provider, project, and time filters.
- Transcripts can be referenced by session ID, path, or `file://` URL.

## v0.2.0 (2026-08-03)

- Added `compactions` to inspect Claude and Codex compaction lifecycle records, usage correlation, and normalized token summaries.

## v0.1.3 (2026-07-24)

- Added `agentscript codex <session-id>` to open a Codex transcript by session ID.
- Discovery honors `CLAUDE_CONFIG_DIR` and `CODEX_HOME`.

## v0.1.2 (2026-07-12)

- Cleaned internal transcript scaffolding out of rendered output.

## v0.1.1 (2026-07-12)

- Codex internal goals are hidden by default.

## v0.1.0 (2026-07-02)

- Initial release: render Claude Code and Codex JSONL transcripts as indexed blocks, with filtering, slicing, analysis, and export workflows.
