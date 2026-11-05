# Ignore foreign Codex activity for an explicitly completed current thread

## Why
An old active marker can name another session while the current bound Codex is complete and ready. Recent window activity wrongly revives working.

## What Changes
Validate the exact current binding/home/cwd and own completion plus a ready nonworking frame before correcting only the display. Preserve markers, bindings, genuine starts/approvals and legacy unknowns.

## Surfaces
- CLI / terminal: shared radar status corrected without input.
- menubar: consumes the same corrected status.
- Web: consumes the same corrected status.
- iPhone / phone: consumes the same corrected status.
- iPad / Android: shared RN consumes it; no native adapters changed.
No workers, release/install, actual marker cleanup or session manipulation.
