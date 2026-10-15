# Tasks

## Implementation

- [x] Add a lazy Codex candidate-path index scoped to one radar gather.
- [x] Reuse the scoped resolver for Codex transcript facts and binding recovery.
- [x] Preserve direct transcript APIs and leave Claude lookup unchanged.
- [x] Cover matching, continuation/archive behavior, freshness, discovery count, and
      concurrent scopes with regression tests.

## Validation

- [x] Run `go vet ./...` and the targeted race tests.
- [x] Complete `env -u CMUX_WORKSPACE_ID make check` (maintainer verification on 2026-10-15).
- [x] Compare baseline and indexed `agents --json` output and runtime samples.
- [x] Confirm the runtime process guard stayed below its stop threshold.
- [x] Record runtime evidence in `docs/test/CODEX-SESSION-PATH-SCAN-2026-10-15.md`.

## Acceptance

The earlier local download and host-dependent failures were followed by a successful
full core gate on the reviewed code (formatting, vet, pinned staticcheck, race tests),
plus all 11 hosted CI checks. The temporary removal of CMUX_WORKSPACE_ID applies only
to this verification process. No global environment was changed.

The change preserves public matching/output behavior and only scopes internal path
discovery, so it needs no capability-spec delta. Native UI, real-device accessibility
and the author-reported timing benchmark were not independently repeated during this
closeout.
