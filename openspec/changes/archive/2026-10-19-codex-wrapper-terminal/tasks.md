## Implementation

- [x] Add a bounded shared foreground/TTY/stdin verifier with platform adapters.
- [x] Use it for wrapper hook candidates and exact Codex delivery binding.
- [x] Keep generic source metadata and document eligibility and old-record limits.
- [x] Verify positive/negative fixtures, valid mutations and relevant gates.
- [x] Sync specs and archive the finished change in the same PR.

## Acceptance

No real model input or synthetic live hook. Runtime checks are read-only metadata checks and do not prove a complete user turn. Author tests are not independent review.

Validation: `make check` passed, including Go race, vet, staticcheck, 61 publisher and 24 CI helper checks. Eleven valid regression mutations were caught and restored. Read-only selected-pane metadata verification passed on final code; no live message or binding was performed. The archive is checked by the final design/spec gate. Direct Codex and generic terminal-input behavior remain unchanged; this qualifies wrapper hook/binding candidates, not an arbitrary API backend input adapter.
