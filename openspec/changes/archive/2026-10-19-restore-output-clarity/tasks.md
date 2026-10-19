## Implementation

- [x] Add compact, width-aware restore output and a bounded detailed preview.
- [x] Keep concise layout warnings and full diagnostic evidence.
- [x] Verify synthetic output, valid regression mutations and required gates.
- [x] Update paired CLI docs, sync the capability spec and archive the change.

## Acceptance

Author checks only; no independent review or live workspace restore is claimed. Tests use synthetic session names and prompts.

Validation: core `make check`, five synthetic output tests and seven valid regression mutations passed. Three opt-in restore contract tests (including seven topology/layout subchecks) passed on private tmux sockets and temporary homes. The archive is validated by the final design/spec gate before commit. No host-terminal focus/order acceptance or independent review.
