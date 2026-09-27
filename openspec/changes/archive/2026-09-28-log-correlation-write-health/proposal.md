# Log correlation and write health

## Why

An HQ action may leave receipts in the event journal, diagnostic log, and knowledge
ledger. Matching by timestamp and pane is ambiguous during concurrent activity.
Diagnostic and event writes also returned silently on filesystem errors, so a
successful action could leave no searchable trace without an operator warning.

## What changes

- Give each audited act a random `op_id` shared by its event and diagnostic
  receipts. Knowledge mutations carry the same ID in their ledger operation.
- Report failed event and diagnostic writes to stderr without including payload
  text, capped at one warning per store per minute. Keep the primary action alive.
- Make `gtmux doctor` test whether the stores can accept an append. Surface a
  failed knowledge-ledger close as an error to the mutation caller.

## Boundary

Legacy records have no operation ID. Ordinary lifecycle events have no paired
diagnostic act. A power loss can still lose recently buffered writes; the probe
checks current writability, not durability across a crash.

## Surfaces

- **terminal / 终端** — doctor reports write health; an append failure warns on stderr;
  JSON output gains `op_id` on audited acts.
- **menubar / 菜单栏** — no UI change; the same diagnostic store gains receipt IDs.
- **Phone** — no UI change; phone-originated audited acts gain receipt IDs on the Mac.
- **iPad** — same behavior as Phone.
- **Web** — no UI change; browser-originated audited acts gain receipt IDs on the Mac.
