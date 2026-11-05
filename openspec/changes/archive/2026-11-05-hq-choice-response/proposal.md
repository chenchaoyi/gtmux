# Make HQ reply choices visible and independent of slow discovery

## Why
The selected matter reads quickly, while roster discovery can exceed ten seconds. The native card updates a draft without focusing it, and existing tests omit the full header/conversation/composer layout.

## What Changes
- Visible prepared state and an explicit composer focus request without auto-send.
- A bounded, scrollable matter header that preserves the pinned composer.
- Fast stored workspace reads; optional roster/history enrichment runs separately and cannot replace canonical matters or drafts. Preserve legacy CLI and unknown/stale source semantics.

## Surfaces
- CLI / terminal: additive stored-only read; ordinary full list remains available.
- menubar: immediate option feedback, composer focus and independent optional sources.
- Web: no new UI or input route; existing protocol contracts remain unchanged.
- iPhone / phone: existing HTTP stored projection and RN reply behavior unchanged.
- iPad / Android: shared RN behavior unchanged; no device installation.
No real HQ answers, destructive cleanup, signing, release, account changes or shared restart.
