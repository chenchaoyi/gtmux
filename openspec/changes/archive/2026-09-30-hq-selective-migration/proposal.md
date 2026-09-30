# Make HQ import and selective migration available from the menu bar

## Why
The HQ records bar exposes only export. CLI import restores the entire old home; the
existing move-between-Macs design has never been implemented. It cannot honestly be
presented as selective migration.

## What changes
- Add visible Restore backup and Move from another Mac actions beside Export.
- Read and validate every archive entry before changing the live home, including bounded
  decompression, private staging, unsafe-path/type/duplicate rejection and decryption.
- Preview and stage only selected long-term content: live knowledge with its lineage,
  LOCAL.md, and non-executable tool attachments. Exclude board/session/control state.
- Review knowledge before atomic ledger merge; refuse conflicting IDs, skip identical
  history, keep source IDs/lineage, and reset old promotion/audience decisions.
- Review LOCAL.md beside current text; keep current by default and require explicit
  replacement with a retained backup. Apply knowledge and personal requirements as
  separate atomic operations, each with a receipt and failure result.
- Require HQ to be stopped for restore/apply; no reset or pane input is sent.

## Surfaces
- terminal: new `hq migrate` preview/stage/review/apply commands; full restore retained.
- menubar: records actions menu and guided archive preview / selection / review sheet.
- phone: existing full copy/export remains; no remote write/import.
- iPad: same full copy/export as phone; no remote write/import.
- Web: no import/export authority; no new endpoints.

## Impact
Go core remains cgo-free. No external services, new releases, real HQ changes, or
credential/config migration. English/Chinese docs describe the implemented limits.
