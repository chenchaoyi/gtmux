## Tasks

- [x] Audit recent Codex-era commits on `main` against `openspec/specs/` to identify un-specced shipped behaviors.
- [x] Author strict OpenSpec deltas for `env-doctor`, `agent-radar`, `mobile-app`, `hq-workspace`, and `chat-transcript` grounded in the merged commits (`dd0d74ef`, `b1a11ef9`, `d0d4acd2`, `e276dd9b`, `9c494080`, `77636f78`, `b277e6fe`, `47d624a2`).
- [x] Validate deltas with `openspec validate --strict` and `./scripts/check-design.sh --source-only`.
- [x] Archive the backfill change into `openspec/specs/` and `openspec/changes/archive/`.
