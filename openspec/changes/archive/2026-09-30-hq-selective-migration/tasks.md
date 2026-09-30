- [x] Implement complete bounded archive validation and safe full restore.
- [x] Implement selective preview/staging and source-preserving knowledge merge.
- [x] Implement explicit LOCAL.md review/replacement and receipts.
- [x] Add visible menu-bar actions and guided preview/review flow.
- [x] Add regression tests for exclusions, sensitive content, collisions, retries,
      corrupt/tampered inputs, failures, UI readiness and layout.
- [x] Sync English/Chinese docs/help/spec and archive this change.
- [x] Run relevant Swift tests, make check, design/spec gates and cgo-free build.
- [x] Review implementation and prepare the PR handoff. CI-gated merge is tracked by the PR.

## Verification evidence
- Full `make check`: passed (format, vet, staticcheck v0.8.1, Go race suite).
- Full macOS `swift test --jobs 2`: 279 tests passed, including 9 import-flow tests.
- AppKit-hosted screenshots inspected for knowledge detail and personal-text comparison;
  all flow steps/tabs rendered in English/Chinese. No installed-app or device acceptance claim.
- Archive early-exit and missing audience-reset mutations both made their regression tests
  fail; mutations restored before final checks.
- Design/spec gate and cgo-free CLI build passed. Synthetic tests only; no real HQ import,
  pane input, release or installation performed. CI results belong to the PR, not this checklist.
