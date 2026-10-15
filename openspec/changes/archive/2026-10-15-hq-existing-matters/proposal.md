# Connect existing HQ work to matters

## Why
The first shipped workspace lists only its new records store. Upgraded homes with a maintained
board and dispatch ledger therefore show zero matters. HQ can only create one by submitting a
new user instruction; its charter teaches workspace replies but never enrollment of existing work.
This makes a populated HQ look empty and encourages reissuing tasks just to get a sidebar row.

## What Changes
- Add HQ-only, transactional registration of an existing goal with explicit provenance and an
  initial HQ assessment, without a user instruction, pending delivery or client operation.
- Teach HQ to reconcile continuing goals on startup and maintain matters for every instruction
  channel; reuse IDs and exact source identities, never infer goals from pane/title similarity.
- Explain the empty state and expose the existing board plus a reviewable request to organize
  current work. Preparing that request preserves drafts and does not send automatically.
- Keep reads free of migration writes; show registration provenance on matter detail.

## Surfaces
- terminal: additive register operation and versioned HQ charter.
- menubar: actionable empty state, draft-safe review request and matter provenance.
- phone: existing board remains unchanged; future workspace consumer shares core data.
- iPad: same current board as phone; no new native layout in this fix.
- Web: current HQ board remains available; no API or guest-access change.

## Boundaries
No automatic Markdown-to-goal conversion or historical grouping guesses, no task replay,
no real HQ migration/rotation/client restart, no release/install during development.
