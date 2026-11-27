## Context

Commits `dd0d74ef` (#1835), `b1a11ef9` (#1889), `d0d4acd2` (#1900), `e276dd9b` (#1902), `9c494080` (#1939), `77636f78` (#1946), `b277e6fe` (#1950), and `47d624a2` shipped production behavior, test designs (`docs/test/designs/*.json`), and user/design documentation without adding corresponding OpenSpec deltas to `openspec/specs/`.

## Goals / Non-Goals

- **Goals**:
  - Capture every user-visible behavioral invariant from those commits in the owning capability specs under `openspec/specs/`.
  - Keep every requirement strictly faithful to the shipped implementation and existing test designs, without inventing unshipped rules.
  - Archive cleanly via `openspec archive` so `openspec validate --strict` and `./scripts/check-design.sh --source-only` pass.
- **Non-Goals**:
  - Modifying production code, tests, or `docs/test/` artefacts.
  - Altering the four still-in-flight OpenSpec changes (`codex-client-lifecycle`, `phone-moves-the-route`, `direct-multi-home`, `mobile-send-receipt-first`).

## Decisions

1. **Map each shipped behavior to its owning capability spec** rather than inventing a new catch-all capability:
   - Input spool admission/occupancy, prepared storage conflict reporting, Codex unverified-binding recovery guidance, and delivery output boundary → `env-doctor`.
   - Participant profile v2 lock sharding/cross-generation reconciliation and radar snapshot/per-pass frame reuse → `agent-radar`.
   - Terminal history retention, amber cached-list freshness banner, and per-thumbnail upload progress/retry → `mobile-app`.
   - Recording confirmed repair work before closing HQ diagnoses → `hq-workspace`.
   - Native `dsh`/`opencode` skill aliases and DSH multi-query previews → `MODIFIED` delta on `Recorded Activity action facts` in `chat-transcript`.
2. **Auditability**: Cite exact merged commit hashes and PR numbers in `proposal.md` so future contributors can trace every backfilled requirement directly to its implementation and test design.
