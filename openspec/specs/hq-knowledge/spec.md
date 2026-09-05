# hq-knowledge Specification

## Purpose

The supervisor's own memory of THIS machine: what it has learned working here, kept as an
append-only ledger with provenance rather than as prose it has to remember.

It is one of three places a rule or a fact can live, and the distinction is what keeps a
lesson from being filed where nobody will receive it:

- the **shipped charter** (`AGENTS.md`, generated from `internal/hq`) belongs to gtmux and
  reaches every machine through a version bump;
- **`LOCAL.md`** belongs to the operator, is never overwritten, and is in context every
  turn — it is how a commander governs their own supervisor without touching code;
- **this ledger** belongs to the machine's supervisor, and is SPENT rather than always
  present: matched entries echo into a dispatch, and the supervisor reads it on purpose.

An entry that turns out to be bigger than one machine leaves through `promote` → a carried
brief → `land`, which is the only step of the whole lifecycle that waits on a person.

Written up for readers in `docs/design/knowledge-layers.md`.

## Requirements

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

The HQ home SHALL be DISCOVERABLE (`gtmux hq --home`) so that a caller which must
mutate can SATISFY the gate rather than be exempted from it: it runs the verb from the
home the CLI names. This is not a widening — the verb is still judged by cwd, the path is
already printed by every refusal, and one resolver keeps a relocatable (and, on real
machines, symlinked) path from being re-derived by every consumer.

#### Scenario: A worker cannot write knowledge

- **WHEN** `gtmux knowledge add` runs from a directory that is not the HQ home
- **THEN** it is refused with a message naming the home, and neither the ledger
  nor any projection changes

#### Scenario: A surface finds the one directory the gate accepts

- **WHEN** a consumer that is about to mutate runs `gtmux hq --home`
- **THEN** it receives the HQ home path, and a `gtmux knowledge` verb run with that
  directory as its cwd passes the gate exactly as the supervisor's own would
- **AND WHEN** no HQ home exists yet
- **THEN** the path is still printed and the command exits non-zero saying so

#### Scenario: A dismissed candidate leaves a trace

- **WHEN** HQ dismisses a pending candidate with a reason
- **THEN** the candidate is gone from the spool, the ledger is untouched, and a
  `gtmux:audit:knowledge` record carries the key and the reason

#### Scenario: The KB's change history is a journal query

- **WHEN** an entry is added, superseded, or retired
- **THEN** one `gtmux:audit:knowledge` record per mutation appears in the
  journal, excluded from the consumption debt like every audit record

### Requirement: A charter-level entry is promoted into an export brief, and the loop closes on landing

A LIVE entry SHALL be promotable as charter-level through
`gtmux knowledge promote <id> --why "…" [--target "…"]` — a ledger operation (the
required `why` states the promotion case; the optional `target` names where the
lesson should land) that writes a PROMOTION BRIEF: a deterministic, gtmux-owned
render under `knowledge/promotions/` carrying the lesson, the why, the target,
the entry's full provenance, and the closing instruction. Charter-level means
the lesson belongs in a DURABLE RULE CARRIER beyond this machine's knowledge
base — a project's `AGENTS.md`/`CLAUDE.md`, a team runbook or wiki, `LOCAL.md`
(when the rule governs this supervisor itself), or gtmux's own repo (an openspec
change or seed edit for a developer, or a GitHub issue carrying the brief for
anyone else). The brief is the EVIDENCE PACKAGE a human — or a worker they
dispatch — carries to that carrier; gtmux SHALL NOT write into any repo or
external system itself, and nothing here dispatches work on its own. The brief's
closing instruction SHALL name the promotion's `target` when one was given, and
otherwise present the carrier options neutrally — never a single hardcoded
destination.

`gtmux knowledge land <id> --ref "…"` SHALL close the loop when the lesson lands
— the reference is whatever names the landing (a PR, an issue URL, a runbook
name, a file path) — removing the brief while the ledger keeps the whole
lifecycle; a later `promote` MAY re-open a landed entry (the lesson evolved). The
write path SHALL refuse promoting a dead or already-pending entry and landing a
dead or never-promoted entry. A SUPERSEDE does not inherit promotion state — the
content changed, so the successor is re-judged. Both operations are mutations
under the knowledge role gate and journal through `gtmux:audit:knowledge`.

`gtmux knowledge promotions` SHALL list the pending queue — read-only, open to
anyone — headed by the count and the OLDEST pending age, and topic renders SHALL
badge a promoted-pending entry and a landed one in the provenance footer, so the
state is visible where the lesson lives.

#### Scenario: A promotion produces a carryable brief

- **WHEN** HQ promotes a live entry with a why and a target
- **THEN** one brief appears under `knowledge/promotions/` carrying the lesson, the
  why, the target, the entry's provenance (seqs/capture/task/pane/date), and a
  closing instruction that names THAT target — and one `gtmux:audit:knowledge`
  record is journaled

#### Scenario: A target-less brief offers the carriers, not a repo mandate

- **WHEN** a promotion carries no `--target`
- **THEN** the brief's closing instruction lists the carrier options (a project
  AGENTS.md/CLAUDE.md, a team runbook, LOCAL.md, gtmux's repo or an issue) rather
  than directing every user into gtmux's source tree

#### Scenario: Landing closes the loop

- **WHEN** the lesson lands in its carrier and HQ runs `land <id> --ref "…"` — the
  ref being a PR, an issue URL, or a runbook name alike
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

### Requirement: The topic vocabulary is extensible through the ledger

The topic vocabulary SHALL be the built-in topics plus every topic DECLARED in
the ledger: `gtmux knowledge topic <name> --desc "…"` appends a `topic`
operation (name + one-line description), a mutation under the knowledge role
gate that journals through `gtmux:audit:knowledge` like every other. A topic
name SHALL be a bounded slug (`[a-z0-9-]`, ≤ 40 bytes) and SHALL refuse —
loudly — a built-in, an already-declared custom, and the reserved names the
directory layout owns (`README`, `legacy`, `promotions`).

One validation over one source SHALL serve every entrance: `gtmux knowledge`
verbs and `gtmux capture` accept exactly the same vocabulary (built-ins,
`environment`, and declared customs — closing the drift where capture accepted
five topics and knowledge six). A declared topic SHALL render immediately
(marker, name, its description as the intro), so the declaration is visible
rather than a silent registry write, and the dispatch-time knowledge echo SHALL
consult every CUSTOM topic alongside the built-in pitfalls/workflows — the
built-in exclusions (accounts, corrections, environment: not dispatch-time
context) stand unchanged. Topic declarations are add-only; removal is deferred
until a real need names it.

#### Scenario: A user's own domain becomes a first-class topic

- **WHEN** HQ declares `gtmux knowledge topic datasets --desc "…"`, a worker runs
  `gtmux capture "… @datasets"`, and HQ accepts the candidate into an entry
- **THEN** the ledger holds the declaration and the entry with its provenance,
  `datasets.md` renders with the description and the lesson, and a later
  dispatch whose repo or goal matches surfaces it in the knowledge echo

#### Scenario: The vocabulary refuses nonsense loudly

- **WHEN** a topic declaration names a built-in, an existing custom, a reserved
  directory name, or an over-long/invalid slug
- **THEN** it is refused with the reason named, and nothing is appended

#### Scenario: Capture and knowledge can no longer disagree

- **WHEN** any topic is judged by `gtmux capture` and by `gtmux knowledge add`
- **THEN** both entrances give the same verdict, from the same vocabulary

### Requirement: The commander can act on knowledge remotely, through a second narrower door

`gtmux serve` SHALL expose the knowledge base to an OWNER-authenticated client, and SHALL
accept from it exactly two mutations: `land` (close a pending promotion with a ref) and
`retire` (remove a live entry with a reason). A GUEST SHALL be refused, like every other
`/api/hq/*` surface.

The CLI's cwd-keyed HQ-home gate SHALL remain unchanged. The two doors exist for two
different callers: the cwd gate keeps WORKERS out of the quality gate, while the serve
door is the COMMANDER, who outranks the supervisor and is the only party who can know that
a promotion actually landed somewhere durable. Both doors SHALL journal the same
`gtmux:audit:knowledge` record, so the change history stays one stream regardless of which
door a mutation came through.

`add` and `supersede` SHALL NOT be exposed remotely. They carry prose, and the quality of
the base is the point of having one.

#### Scenario: A guest cannot see or write knowledge

- **WHEN** a client holding a guest token requests any knowledge endpoint
- **THEN** the request is refused `403`, and no entry, title or count is disclosed

#### Scenario: The commander closes a promotion from the phone

- **WHEN** an owner-authenticated client posts `land` for an entry with a pending
  promotion, carrying a ref
- **THEN** the ledger records the landing, the promotion brief is removed, the topic
  render is refreshed, and one `gtmux:audit:knowledge` record is journaled — identically
  to the same verb run from the HQ home

#### Scenario: Landing something that was never promoted is refused

- **WHEN** `land` names an entry with no pending promotion
- **THEN** the request fails with a message saying so, and the ledger is untouched

### Requirement: The knowledge read API separates the index from the entry

The read surface SHALL be two endpoints: an INDEX carrying every live entry's identity,
topic, title, timestamp, promotion state and provenance summary but NOT its body, and a
DETAIL endpoint carrying one entry's body.

The split is a size judgment, not a convenience: a base of a few hundred entries with
bodies is megabytes over a tunnel, while the same index without them is tens of kilobytes.
The index SHALL also report the topic vocabulary with per-topic counts, the pending
promotion count with the age of the oldest, and the pending capture-candidate count — the
three figures that say whether the base is being maintained.

#### Scenario: The index is small enough to poll

- **WHEN** the index is requested for a base of several hundred entries
- **THEN** the response carries every entry's identity and state without bodies

#### Scenario: A base nobody has written to is ordinary

- **WHEN** the index is requested on a machine with no HQ home or an empty ledger
- **THEN** the response is `200` with empty collections and zero counts, not an error
