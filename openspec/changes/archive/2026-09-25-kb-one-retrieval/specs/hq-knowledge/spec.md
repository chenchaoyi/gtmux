# hq-knowledge (delta)

## ADDED Requirements

### Requirement: One retrieval answers every question asked of the base

gtmux SHALL have exactly one implementation of what it means for a piece of text to match a
knowledge entry: one tokenizer, one set of corpus statistics, one score. `knowledge
neighbours`, `knowledge search`, the entries `add` shows before it writes, and the echo
printed at dispatch SHALL all use it.

Scoring SHALL weight a shared token by how rare it is across the live base, so that a token
two entries share counts for more when few other entries carry it.

The tokenizer SHALL work in both languages: ASCII words, and CJK character bigrams for text
that has no spaces.

#### Scenario: A rare word outranks a common one

- **WHEN** two entries each share one token with the query, one token carried by most of
  the base and one carried by a handful of entries
- **THEN** the entry sharing the rare token ranks higher

#### Scenario: No second opinion about what a match is

- **WHEN** a caller inside the knowledge package needs to find related entries
- **THEN** it calls the shared retrieval, and no caller keeps its own tokenizer or its own
  matching rule

### Requirement: The base can be asked a question in words

`gtmux knowledge search "<text>"` SHALL rank live entries against that text and print them
with their id, kind and title, `--json` for a structured answer, and SHALL accept `--topic`
and `--kind` to narrow the search. It SHALL read the ledger and SHALL work from any
directory.

#### Scenario: A question in Chinese

- **WHEN** HQ runs `gtmux knowledge search "手机配对超时"`
- **THEN** the entries about pairing timeouts rank first, because the tokenizer splits CJK
  text that carries no spaces

### Requirement: Knowledge reaches the moment work starts

At dispatch, gtmux SHALL print the live entries that match the target repository and the
goal, ranked by the shared retrieval rather than by substring, so that a goal written in
Chinese returns hits. The echo SHALL cover `pitfalls`, `workflows`, `best-practices` and
every declared custom topic, and SHALL NOT cover `accounts`, `corrections` or
`environment`. It SHALL stay capped at a few lines, SHALL be advisory, and SHALL print
nothing when nothing matches.

#### Scenario: A goal with no spaces in it

- **WHEN** HQ spawns work with the goal `修复手机端配对超时`
- **THEN** the pairing entries are echoed, where a substring match over the same goal
  returned nothing

#### Scenario: A topic kept out of the echo

- **WHEN** an entry under `corrections` matches the goal's words
- **THEN** it is not echoed, because a correction is not dispatch-time context

## MODIFIED Requirements

### Requirement: The ledger is linted and neighbours are found without a model

`knowledge lint` SHALL report orphans, broken `[[links]]`, near-duplicate titles, stale
entries (superseded-by pending, hypothesis past its floor, promoted past its floor unless
`everyone`), and assumed kinds; it SHALL never edit. It SHALL run inside self-check.
`knowledge neighbours` SHALL rank entries by kind then by the shared retrieval's score and
SHALL return ten; `capture --list` SHALL group the pool by neighbourhood; `add` SHALL show
the closest three live entries.

`knowledge lint` SHALL additionally report, from the writing-rules table: a guess presented
as a fact, a title that the body's first sentence restates, and an implementation name
standing as a title.

#### Scenario: Eleven leads are one lesson

- **WHEN** eleven pool candidates share a neighbourhood
- **THEN** `capture --list` shows them as one group with their keys, so a single
  `add --capture k1,…,k11` files them as one entry with every provenance

#### Scenario: A title made of identifiers

- **WHEN** an entry is titled `hqNudge 为 false 时 sourceKey 为空`
- **THEN** the lint names both identifiers, because the reader who most needs the title is
  the one who cannot resolve them yet
