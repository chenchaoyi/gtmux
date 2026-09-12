# hq-knowledge (delta)

## MODIFIED Requirements

### Requirement: Knowledge is an append-only ledger of entries with provenance

Every live entry SHALL carry three orthogonal fields: a `kind` from the vocabulary
`facts | howto | pitfalls | judgment | decisions`; a `provenance` naming its source
(`correction | recurrence | mined | capture | self`) with a count and last-seen time; and,
once promoted, an `audience` from `hq | machine | repo | everyone`. `topic` SHALL remain
as free tags. A status `hypothesis` SHALL exist for a filed but unverified lead; it
renders in its own section and is never distributed. Records written before this
change SHALL be mapped at read time by a fixed table and flagged by lint until confirmed;
no record is rewritten in place.

#### Scenario: A legacy corrections entry is read

- **WHEN** a v1 record with `topic: corrections` is read
- **THEN** it reads as `kind: judgment`, `provenance: correction`, and lint reports
  "kind assumed at migration" until `knowledge kind` or `supersede` confirms it

#### Scenario: A filed lesson is hit again

- **WHEN** the miner or the correction lexicon matches a live entry's neighbourhood
- **THEN** the entry's provenance count increments and the recurrence is visible on the
  entry, in `lint`, and in the distill brief

### Requirement: A charter-level entry is promoted into an export brief, and the loop closes on landing

`promote` SHALL require `--for <audience>` and SHALL refuse free-text targets. The brief
SHALL contain the ready-to-paste block for that audience. `withdraw <id> --why` SHALL
return a promoted entry to live, journaled. A brief with audience `everyone` SHALL NOT
count toward the overdue floor.

#### Scenario: HQ promoted something the commander decides to keep local

- **WHEN** `knowledge withdraw <id> --why "…"` runs on a promoted entry
- **THEN** the entry is live again, the brief is removed, the withdrawal is journaled,
  and `retire` was not needed

## ADDED Requirements

### Requirement: Knowledge is distributed to the agents who must know it

For audience `machine`, gtmux SHALL render one canonical file under the config dir and
maintain a sentinel-delimited managed block in each supported agent's global instruction
file carrying an INDEX (one line per entry + the canonical path), never the full text.
For `repo`, gtmux SHALL maintain the same block, with full text, in the repository's
`AGENTS.md` (or `CLAUDE.md` when only that exists) and SHALL NOT commit. For `everyone`,
gtmux SHALL produce a prefilled issue URL and a copyable brief. Writes SHALL touch only
the inside of the block. `knowledge sync` SHALL refresh every carrier.

#### Scenario: A user hand-edits inside the block

- **WHEN** the block's content no longer matches its hash
- **THEN** `doctor` reports "knowledge sync: <agent> hand-edited", `sync` refuses to
  overwrite without `--force`, and nothing outside the block is ever touched

#### Scenario: An agent's instruction file does not exist

- **WHEN** the carrier file for a supported agent is absent
- **THEN** `sync` creates it containing only the block, and doctor shows it in sync

### Requirement: The ledger is linted and neighbours are found without a model

`knowledge lint` SHALL report orphans, broken `[[links]]`, near-duplicate titles, stale
entries (superseded-by pending, hypothesis past its floor, promoted past its floor unless
`everyone`), and assumed kinds; it SHALL never edit. It SHALL run inside self-check.
`knowledge neighbours` SHALL rank entries by kind then keyword overlap; `capture --list`
SHALL group the pool by neighbourhood; `add` SHALL show the closest three live entries.

#### Scenario: Eleven leads are one lesson

- **WHEN** eleven pool candidates share a neighbourhood
- **THEN** `capture --list` shows them as one group with their keys, so a single
  `add --capture k1,…,k11` files them as one entry with every provenance

### Requirement: distill never collapses the base

A distill pass SHALL only apply itemized operations (`add`, `supersede`, `retire`,
`dismiss`); it SHALL NOT rewrite a topic or canonical file wholesale, and `supersede`
SHALL keep the superseded text retrievable.

#### Scenario: A merge shortens an entry

- **WHEN** `supersede` replaces an entry with a shorter one
- **THEN** the previous text remains in the ledger and `show <old-id>` returns it
