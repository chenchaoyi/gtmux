## Why

Follow settings overrides the native enabled switch with grey, requires Save/Cancel after a toggle, and inserts/removes optional rows and receipts. The supplied phone screenshot makes enabled states unclear and the sheet jumps as the master permission changes.

## What Changes

Use the same native switch appearance as App settings. Apply each toggle immediately with revision checking and honest confirmation/error handling. Keep all three options mounted; disable subordinate options until HQ follow is confirmed. Replace save/cancel footer with Done in the header and a fixed status slot.

## Surfaces

- terminal: policy API and permissions unchanged.
- menubar: existing follow form unchanged.
- phone: instant settings, native switches and stable option layout.
- iPad: same shared sheet behavior and bounded layout.
- Web: unchanged.

## Impact

Mobile follow sheet and tests, mobile-app spec, bilingual mobile design docs. No server/API migration; policy revision and child permission clearing are preserved.
