# Compact working-state disclosure

## Why
The raw current-screen card distracts from readable conversation progress. Readers need a clear working/thinking row with elapsed time, and an intentional way to inspect or hide the screen.

## What changes
Default to a borderless, compact status disclosure. Show explicit Show live view / Hide live view text and a chevron, retain elapsed time, and mount a bounded scrollable preview only on demand. Retain the choice during refresh, reset it on a new working turn or conversation, and keep pending-input interaction independent.

## Surfaces
- Terminal: not applicable; raw terminal behavior and input stay unchanged.
- Menu bar: not applicable; the native external reader has no current-screen preview.
- Phone: use the shared ChatView disclosure in Detail and HQ.
- iPad: the same component in the regular shell, without a separate implementation.
- Web: not applicable; the browser currently has no automatic working-screen card. Its explicit Interact panel and waiting choices retain their existing disclosure.
