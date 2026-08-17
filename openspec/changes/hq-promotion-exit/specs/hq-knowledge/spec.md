# hq-knowledge (delta)

## ADDED Requirements

### Requirement: A charter-level entry is promoted into an export brief, and the loop closes on landing

A LIVE entry SHALL be promotable as charter-level through
`gtmux knowledge promote <id> --why "…" [--target "…"]` — a ledger operation (the
required `why` states the promotion case; the optional `target` suggests where in
the gtmux repo it belongs) that writes a PROMOTION BRIEF: a deterministic,
gtmux-owned render under `knowledge/promotions/` carrying the lesson, the why, the
target, the entry's full provenance, and the closing instruction. The brief is the
EVIDENCE PACKAGE a human — or a worker they dispatch — carries into the gtmux repo;
gtmux SHALL NOT write into any repo itself, and nothing here dispatches work on its
own.

`gtmux knowledge land <id> --ref "…"` SHALL close the loop when the lesson lands
(a PR / spec / seed reference), removing the brief while the ledger keeps the whole
lifecycle; a later `promote` MAY re-open a landed entry (the lesson evolved). The
write path SHALL refuse promoting a dead or already-pending entry and landing a
dead or never-promoted entry. A SUPERSEDE does not inherit promotion state — the
content changed, so the successor is re-judged. Both operations are mutations under
the knowledge role gate and journal through `gtmux:audit:knowledge`.

`gtmux knowledge promotions` SHALL list the pending queue — read-only, open to
anyone — headed by the count and the OLDEST pending age, and topic renders SHALL
badge a promoted-pending entry and a landed one in the provenance footer, so the
state is visible where the lesson lives.

#### Scenario: A promotion produces a carryable brief

- **WHEN** HQ promotes a live entry with a why and a target
- **THEN** one brief appears under `knowledge/promotions/` carrying the lesson, the
  why, the target, the entry's provenance (seqs/capture/task/pane/date), and the
  `land` instruction — and one `gtmux:audit:knowledge` record is journaled

#### Scenario: Landing closes the loop

- **WHEN** the lesson lands in the gtmux repo and HQ runs `land <id> --ref "#812"`
- **THEN** the brief is removed, the entry's render badge flips to landed with the
  reference, the pending queue no longer names it, and the ledger still holds both
  operations

#### Scenario: The lifecycle refuses nonsense states

- **WHEN** a promote targets an already-pending entry, or a land targets a
  never-promoted one
- **THEN** the operation is refused loudly and nothing is appended

#### Scenario: A successor is re-judged

- **WHEN** a promoted entry is superseded
- **THEN** the successor carries no promotion state — promoting it again is a fresh
  judgment

### Requirement: Pending promotions are visible in doctor

`gtmux doctor`'s HQ maintenance section SHALL report the promotions queue: quiet
(OK) when it is empty or every pending brief is younger than the staleness floor,
and FLAGGED with the count and the oldest age once the oldest pending brief has
stood past it — because a promotion nobody carries is exactly the charter-flags rot
this mechanism exists to end, and it is otherwise silent. The distill ritual's
teaching SHALL include checking the queue, so the existing periodic clock keeps it
warm without a new wake class.

#### Scenario: A stale pending promotion is flagged

- **WHEN** a promotion has been pending past the staleness floor
- **THEN** the doctor row is flagged, naming the pending count and the oldest age

#### Scenario: An empty or fresh queue stays quiet

- **WHEN** no promotion is pending, or the oldest is younger than the floor
- **THEN** the row reports OK and nags nobody
