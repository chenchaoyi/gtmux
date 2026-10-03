## Implementation

- [x] Match resumed Codex rollout files by exact session metadata and aggregate turn boundaries.
- [x] Read first/last activity and Chat turns across matching rollouts; invalidate Chat ETags when a continuation grows.
- [x] Add regression tests for completion, abort, a later start, and a lookalike file.
- [x] Update the Codex integration documentation in English and Chinese.

## Verification

- [x] Run focused Go tests, `make check`, `scripts/check-design.sh`, and strict OpenSpec change validation.
- [x] Confirm the reported session became idle at 15:30:59 via the built core's `GatherAgents()`; a new `task_started` at 15:52:56 correctly put it back in working state in the live CLI.
- [x] Open a PR, wait for green CI, and merge. (#1247)
