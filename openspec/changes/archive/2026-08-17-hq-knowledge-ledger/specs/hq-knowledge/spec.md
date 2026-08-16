# hq-knowledge (delta)

## ADDED Requirements

### Requirement: Knowledge is an append-only ledger of entries with provenance

The knowledge base's authority SHALL be an append-only ledger
(`~/.config/gtmux/hq/knowledge/.ledger.jsonl`): one JSON operation per line, each
carrying a schema version, an operation (`add` / `supersede` / `retire`), a stable
entry id (`<topic>/<slug>` derived from the title), the topic, a single-line
title, an optional markdown body, a timestamp, and PROVENANCE — the event
sequence stamped at write time, an optional sequence range (the distill delta the
entry came from), and, when the entry consumed a capture candidate, that
candidate's key, pane, task, and sequence, inherited rather than discarded.

The LIVE set SHALL fold from the ledger: an `add` makes an entry live, a
`supersede` makes its predecessor dead and the successor live (the predecessor's
operation remains in the ledger — history is the point), and a `retire` makes an
entry dead with a required reason. A malformed ledger line SHALL be skipped, not
fatal.

Bounds SHALL refuse LOUDLY: a multi-line or over-budget title (200 bytes), an
over-budget body (8 KiB), or an over-budget reason (300 bytes) is an error naming
the limit — never a silent truncation, because knowledge is curated content and
truncation corrupts it. An `add` whose id collides with a LIVE entry SHALL be
refused with a message naming the supersede alternative, never merged by guess.

#### Scenario: A capture candidate's provenance survives into the entry

- **WHEN** `gtmux knowledge add --topic pitfalls --title "…" --capture <key>`
  consumes a pending candidate that carried pane `%21`, seq 6650, and a task id
- **THEN** the appended ledger operation records that pane, seq, task, and
  capture key, and the candidate is removed from the pending spool

#### Scenario: A supersede keeps the history

- **WHEN** an entry is superseded
- **THEN** the successor is live, the predecessor is absent from the live set, and
  BOTH operations remain readable in the ledger

#### Scenario: An over-budget body refuses loudly

- **WHEN** an `add` is attempted with a body over the budget
- **THEN** the command errors naming the limit and appends nothing

#### Scenario: An id collision names the alternative

- **WHEN** an `add` derives an id equal to a LIVE entry's
- **THEN** it is refused with a message naming `supersede <id>` (or a retitle),
  and nothing is appended

### Requirement: Topic files are deterministic renders of the live entries

Each `knowledge/<topic>.md` SHALL be a deterministic render of that topic's live
entries, headed by a gtmux-owned marker naming the ledger as the authority and
the verbs as the edit path. Entries SHALL render in ledger order as bullet lines
(a short body flattened into the bullet; a long body indented beneath it), each
with a provenance footer (id, date, sequence evidence, capture/task/legacy
source). Rendering SHALL be idempotent, and `gtmux knowledge render --check`
SHALL report a projection that no longer matches its render — a hand edit is
DETECTABLE drift, not silently absorbed and not silently overwritten.

#### Scenario: A hand-edited projection is caught, not absorbed

- **WHEN** a rendered topic file is edited by hand and `render --check` runs
- **THEN** the drift is reported with the file named and a nonzero exit; a clean
  tree passes silently

#### Scenario: Rendered entries stay consultable

- **WHEN** a live entry's title or flattened body carries a repo name or goal
  keyword
- **THEN** the dispatch-time knowledge echo matches it exactly as it matched
  hand-written bullets

### Requirement: Existing hand-written topics migrate to legacy on first touch, and consult covers both

The first mutation touching a topic whose file predates the ledger SHALL move
that file VERBATIM to `knowledge/legacy/<topic>.md` and link it from the new
render; a file still byte-equal to its seeded placeholder SHALL be replaced
outright (there is nothing to preserve). Migration SHALL be idempotent — a second
mutation never re-migrates. The dispatch-time knowledge echo SHALL consult the
renders AND the legacy files, so no captured lesson loses reach during the
incremental migration; the distill teaching directs HQ to migrate the legacy
lessons it touches (an ordinary `add` whose provenance names `legacy`), so the
legacy files shrink by use.

#### Scenario: Hand-written content survives byte-for-byte

- **WHEN** the first `add` lands on a topic with 200 KB of hand-written lessons
- **THEN** those bytes are moved unchanged to `legacy/<topic>.md`, the render
  links to it, and a keyword that matched a legacy bullet before the migration
  still matches after

#### Scenario: A placeholder is not worth preserving

- **WHEN** the first `add` lands on a topic whose file equals its seeded
  placeholder
- **THEN** the placeholder is replaced by the render, and no legacy file appears

### Requirement: Knowledge mutations are supervisor verbs, journaled

`gtmux knowledge` SHALL be a public command whose MUTATIONS (`add`, `supersede`,
`retire`, `dismiss`, `render`) are accepted only from the HQ home — the same
cwd-keyed role rule as `gtmux events --ack`, refused loudly elsewhere — because
the quality gate is the supervisor; `list` and `show` SHALL work anywhere.
`dismiss --capture <key> --why` SHALL remove a pending candidate WITHOUT a ledger
operation but WITH a trace: the quality gate's rejections are evidence, not
silence. Every mutation SHALL append one `gtmux:audit:knowledge` journal record
(an audit record under the session-events audit rules: trail, not debt), so the
knowledge base's change history rides the same stream as everything else.

#### Scenario: A worker cannot write knowledge

- **WHEN** `gtmux knowledge add` runs from a directory that is not the HQ home
- **THEN** it is refused with a message naming the home, and neither the ledger
  nor any projection changes

#### Scenario: A dismissed candidate leaves a trace

- **WHEN** HQ dismisses a pending candidate with a reason
- **THEN** the candidate is gone from the spool, the ledger is untouched, and a
  `gtmux:audit:knowledge` record carries the key and the reason

#### Scenario: The KB's change history is a journal query

- **WHEN** an entry is added, superseded, or retired
- **THEN** one `gtmux:audit:knowledge` record per mutation appears in the
  journal, excluded from the consumption debt like every audit record
