# Preserve knowledge sources and settle candidates reliably

## Why

Accepting a capture currently rewrites the pending spool before the knowledge ledger
is appended. A later validation or storage failure loses the pending work. Only keys,
event sequence numbers and the last pane/task survive acceptance; transcript context,
session and project do not. Dismissals retain only a bounded event summary.

## What changes

- Give candidates stable identities and content digests; retain all available source
  metadata and excerpts in the accepting or dismissing ledger operation.
- Treat one durable ledger record as the settlement point. The pending view excludes
  settled identities; source records are never removed before a commit.
- Serialize selection and settlement across processes and preserve the previous file
  on failed writes. Distinguish an uncommitted failure from a committed operation whose
  derived views need repair.
- Expose sources on detail reads and committed settlements through `knowledge receipts`.
  Emit correlated, structured audit outcomes without copying source text into diagnostics.

This batch does not change knowledge ranking, applicability, extraction policy, agent
task injection, promotion authority, or add a database/model service.

## Impact

Additive ledger/JSON fields; legacy records remain readable without fabricated evidence.
Dismissals become non-content ledger records, excluded from live entries and distribution.
Source excerpts stay in the local records and owner detail reads, never generated global
instructions or public promotion briefs. English and Chinese docs change together.

## Surfaces

- terminal (including attach): adds `knowledge receipts`, source detail JSON and honest commit failures; attach itself is unchanged.
- menubar: shared mutations retain sources; no layout change in this batch.
- phone: owner entry detail gains optional sources; existing clients remain compatible. No UI change in this batch.
- iPad: same shared owner detail contract as Phone, without a layout change.
- Web: no raw source evidence in share pages or indexes; no UI change.
