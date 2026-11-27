## Why

Add full Tier 2+ integration for Google Antigravity CLI (`agy`) across registry detection, native hook/status handling, usage/context-window telemetry, doctor/setup diagnostics, transcripts, HQ knowledge distribution, and bilingual docs. This archive record backfills the change history for PR #1959 (`5751baf8`), whose capability specs were synced directly into `openspec/specs/agent-integration/spec.md` and `openspec/specs/hq-knowledge/spec.md`.

## What Changes

- Register `agy` (`Antigravity`) as a Tier 2+ agent in `internal/agents/registry.go`, `mobileapp/e2e/fake-serve/icons.ts`, and macOS/web/mobile metadata.
- Wire native hook/setup/doctor checks, usage & context-window backfill, transcript support, and HQ knowledge sync (`~/.gemini/antigravity-cli/skills` / `GEMINI.md`).
- Update `openspec/specs/agent-integration/spec.md`, `openspec/specs/hq-knowledge/spec.md`, and bilingual user/design docs.

## Surfaces

- **CLI**: `gtmux agents`, `gtmux doctor`, `gtmux setup`, `gtmux usage`, `gtmux hq`, `gtmux transcript` — shipped in `5751baf8`.
- **Web UI**: Agent badge/icon and transcript/activity display — shipped in `5751baf8`.
- **Mobile app**: Agent icon/key mapping and fake-serve e2e fixtures — shipped in `5751baf8`.
- **macOS menu-bar app**: Agent list/icons and doctor integration — shipped in `5751baf8`.
- **Docs**: `docs/agents.md`, `docs/agents.zh.md`, `docs/cli.md`, `docs/cli.zh.md`, `docs/usage.md`, `docs/usage.zh.md`, `openspec/specs/agent-integration/spec.md`, `openspec/specs/hq-knowledge/spec.md` — shipped in `5751baf8`.
