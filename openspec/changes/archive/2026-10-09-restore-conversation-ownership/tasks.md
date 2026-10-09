## Implementation
- [x] Implement reserved IDs, locator exclusion and ambiguity refusal.
- [x] Align restore preview and bilingual user receipts/docs.
- [x] Verify regression tests, make check and strict change validation.

## Acceptance
No release or installation. Existing live clients are intentionally left running; this change prevents future incorrect restore selection. Phone/iPad physical acceptance is not a claim of this core-only fix.

## Evidence
- Selection/preview regression tests and four isolated real tmux cases passed.
- make check passed (fmt, vet, pinned staticcheck and full race suite).
- Operator %18/%19 were read only; no input, reset or termination was sent.
