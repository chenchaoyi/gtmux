## Why

During a rapid Codex-assisted development window, several user-visible behaviors shipped to `main` with full test designs, code, and bilingual docs, while their corresponding `openspec/specs/` contracts were either skipped or left unarchived in `openspec/changes/` behind post-install manual acceptance checkboxes. Backfilling these shipped contracts restores `openspec/specs/` as the complete source of truth for behavior across all agent harnesses.

## What Changes

Backfill specifications for seven shipped behaviors (without altering any runtime code):
- `env-doctor`: active terminal input spool occupancy reporting and 8,192-slot legacy backlog admission (`e276dd9b` / #1902), prepared storage fence conflict error reporting (`b277e6fe` / #1950), read-only safe recovery guidance for unverified Codex bindings (`77636f78` / #1946), and external-destination protection for release/Store delivery outputs (`d0d4acd2` / #1900).
- `agent-radar`: bounded v2 participant-profile lock sharding with cross-generation model/effort reconciliation (`d0d4acd2` / #1900), and fresh SSE-to-REST snapshot reuse with per-pass pane frame sharing that keeps approval verification fresh (`dd0d74ef` / #1835).
- `mobile-app`: 72pt per-thumbnail upload status and guarded manual retry, rejection of blank saved upload paths, amber cached-list freshness notice with bounded retry deadline, and terminal scrollback retention while reading (`b1a11ef9` / #1889).
- `hq-workspace`: recording confirmed, authorized, unclaimed repair defects before closing HQ diagnoses while preserving the original external hold (`9c494080` / #1939).
- `chat-transcript`: refining `Recorded Activity action facts` to include native `dsh`/`opencode` skill identity aliases (`skill`/`name`/`id`) and bounded DSH multi-query search previews (`47d624a2`).

## Surfaces

- **CLI**: `gtmux doctor`, `gtmux doctor --no-prompt`, `gtmux serve`, `gtmux hq` (backfilled specs only; code already shipped).
- **Web UI**: Activity / Trace native skill & web-search labels (backfilled spec only; code already shipped).
- **Mobile app**: Terminal history retention, amber cached-list freshness banner, and 72pt per-thumbnail upload progress/retry (backfilled spec only; code already shipped).
- **macOS menu-bar app**: Shares Activity / Trace action labels and radar snapshot endpoints (backfilled spec only; code already shipped).
- **Docs**: Bilingual design and CLI docs already shipped in the cited commits; this change aligns `openspec/specs/` with them.

## Capabilities

### Modified Capabilities
- `env-doctor`: Add requirements for active input spool admission/occupancy, prepared storage fence conflict diagnostics, read-only Codex recovery guidance, and delivery output path boundaries.
- `agent-radar`: Add requirements for bounded v2 participant profile storage and fresh snapshot/per-pass pane frame reuse.
- `mobile-app`: Add requirements for terminal history retention, amber cached-list refresh feedback, and per-thumbnail upload progress/retry.
- `hq-workspace`: Add requirement for recording confirmed repair work before closing HQ diagnoses.
- `chat-transcript`: Update `Recorded Activity action facts` with native `dsh`/`opencode` skill identity aliases and DSH multi-query search previews.

## Impact

- Spec-only backfill under `openspec/`; zero runtime code, API, or test behavior changes.
- All behaviors documented here are already implemented, tested, and merged on `main`.
