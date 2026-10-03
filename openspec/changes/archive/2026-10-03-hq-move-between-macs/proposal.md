# Move selected HQ knowledge to a new Mac

## Why

The existing HQ archive is a complete restore. It contains the old situation board and
the shipped `AGENTS.md`, as well as the knowledge ledger and the user's `LOCAL.md`.
Restoring the entire archive onto a different Mac would make HQ read old sessions as the
new Mac's current situation and replace instructions that the installed gtmux should own.
The phone's explanation also calls `LOCAL.md` “Your standing rules” and `AGENTS.md` “The
charter”, leaving their practical difference unclear.

## What changes

This change clarifies the records explanation now. A separate **Move from another Mac**
flow is designed in [the bilingual design](../../../docs/design/hq-move-between-macs.md)
for later implementation. It accepts an existing encrypted Mac export or the phone's
unencrypted full copy as input, previews
it read-only, and lets the user select the knowledge base and/or their own instructions.
The old board and generated instructions are never imported by the move flow. Knowledge
from the old Mac enters a review state instead of becoming new-machine truth on arrival.
Same-Mac `gtmux hq --import` remains a complete restore.

## Surfaces

- **终端 / terminal / attach**: a future `gtmux hq migrate --from` offers preview and
  selective apply. Attach and the current full restore are unchanged.
- **菜单栏 / menubar**: a future records action opens a file picker, preview, selection
  and receipt. The existing export continues to make a full recovery archive.
- **手机 / phone**: Settings now explains the four contents and their owners plainly.
  The existing phone copy can be exported to Files and later chosen on the new Mac.
- **iPad**: shares the Settings explanation and copy/export flow with the phone.
- **Web**: no records export or import; shared pages remain outside this workflow.

## Boundary

This PR designs the move flow and corrects the explanation. It does not expose a partial
import until ledger merge, conflict handling, sensitive-entry review and rollback are
implemented and verified. The current `--import` is still for full recovery.
