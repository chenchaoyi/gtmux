# slim-attention-ledger — the ledger keeps the verbs it uses, loses the ones it never wired

Origin: the commander's whole-of-hq review flagged the attention ledger's
dangling writes; this clears that debt.

## Why — the measured findings

1. **Five write verbs, zero callers.** `Promote`, `SetPriority`, `MarkSurfaced`,
   `ArchiveTask`, `ListArchived` are implemented and tested in
   `internal/dispatch` — and no CLI, sensor, or subsystem ever calls them. The
   spec's "late promotion" and "archived entries stay retro-queryable"
   scenarios cannot happen: HQ, the only intended writer, has no command to
   perform them.
2. **Three fields with display logic and no data.** `Tier`, `Priority`,
   `Surfaced` render in `gtmux tasks --verbose` and are forever
   empty/zero/false; `--verbose` appends archived rows from a directory
   nothing ever writes. The one non-display read of `Archived` is a filter
   condition that is constantly false.
3. **The playbook teaches machinery that does not exist.** "a QUIET item …
   can be PROMOTED later" and the self-check's "ledger archival" name
   operations HQ cannot run — the same teaching-points-at-nothing disease
   `hq-feed` had.
4. **The live set already stays small by a different mechanism.** `gtmux reap`
   REMOVES settled entries; archiving was designed as the rollability story
   and was never needed.

## What changes

- **The unwired verbs and their fields go**: `Promote`/`SetPriority`/
  `MarkSurfaced`/`ArchiveTask`/`ListArchived`, the archive directory, and the
  `Tier`/`Priority`/`Surfaced`/`SurfacedAt`/`Archived`/`ArchivedAt` fields on
  Task and its JSON row. A legacy ledger file carrying them still loads
  (unknown JSON fields are ignored).
- **What is actually used stays**: the awaiting-commander disposition closed
  loop (`--await`/`--resolve [disposition]`, the `--pending` view,
  `AwaitingSince`), and `--verbose` keeps its real job — disposition detail on
  the live set.
- **The playbook stops teaching ghosts** (QUIET items are recorded and
  retro-queryable — no promotion story; self-check's ledger duty becomes
  settling stale pending entries), v29 → v30. docs/cli.md's self-check row and
  CLAUDE.md's ledger summary follow.
- **Specs**: the `Attention ledger` requirement is REMOVED (its surviving
  content — the disposition loop — already lives in `Pending-decision standing
  view`, which is MODIFIED to carry the `--verbose` detail promise and drop
  the archiving language); supervisor-agent's self-check requirement drops
  "ledger archival" (and a leftover "feed health").

## Non-goals

The pending-decision standing view, reap's settle-and-remove lifecycle, and
the wake/watermark machinery are untouched. No new ledger verbs: if a real
late-promotion need ever appears, it starts from a use case, not from
restoring dead code.

## Impact / risk

Deletion-only on the code side; the JSON row shrinks by fields that were
always empty (no consumer reads them — checked macapp, mobileapp,
api/contract.md). Old on-disk entries with the removed fields keep loading.
