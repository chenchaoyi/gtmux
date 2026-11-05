# HQ overview clarity

## Why
The native overview hides the remaining list behind a header action and shows stage receipts without a clear whole-goal status. Completed records also require HQ reconciliation, not client-side guessing.

## What Changes
- Add a remaining-count footer that opens the existing full list.
- Bound/fill canonical previews and exclude ended/merged records.
- Show current assessment and whole-goal status separately from session status.
- Strengthen HQ closure/blocker summary guidance without inventing human decisions.

## Surfaces
- CLI / terminal: managed HQ instructions only, no protocol/state change.
- menubar: overview presentation and native tests.
- Web: no new workspace surface or protocol.
- iPhone / phone: existing remaining footer is the interaction reference.
- iPad / Android: shared RN unchanged; no native adapter changes.
Worker behavior is unchanged.
