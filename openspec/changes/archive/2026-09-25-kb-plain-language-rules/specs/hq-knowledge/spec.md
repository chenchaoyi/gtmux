# hq-knowledge (delta)

## ADDED Requirements

### Requirement: The writing rules are one table, and every reader takes it from there

gtmux SHALL carry the rules for how a knowledge entry reads as a single table in the code.
Each rule SHALL state its number, what to do in one line, why, and a before-and-after pair
of real sentences, in both English and Chinese, each half written for that language's
reader rather than translated.

Each rule SHALL declare whether it is mechanical (a matcher can find it) or a judgement
(only a reader can settle it), and, for a mechanical rule, whether one sighting is enough
(strong) or whether it counts only alongside another (weak).

`gtmux knowledge style` SHALL print the table, and `--json` SHALL give it structured for an
agent. The lint SHALL take its mechanical rules from the same table, and the seeded playbook
SHALL point at the command rather than restate the rules.

#### Scenario: The lint and the playbook cannot disagree

- **WHEN** a rule's wording or tier changes in the table
- **THEN** `gtmux knowledge style`, `gtmux knowledge lint` and the playbook's pointer all
  reflect that one edit, because none of them holds a second copy

#### Scenario: An agent asks what the rules are before writing

- **WHEN** an agent runs `gtmux knowledge style --json`
- **THEN** it gets every rule with its tier, its one-line instruction and its example pair

### Requirement: Plain language never costs an agent what it needs to comply

The rules SHALL state that everything executable stays verbatim: commands, paths,
thresholds, ids, error text, dates, numbers and the commander's own words. A rewrite SHALL
change the saying and never the fact.

The rules SHALL state that an entry says who, where, what happened and what to do, so that
an entry can be acted on and not only understood.

The `ai-voice` check SHALL continue to read prose only, skipping fenced and inline code,
indented blocks, table rows and quoted spans.

#### Scenario: A pitfall entry carrying a command

- **WHEN** an entry's body quotes `command rm -f <path>` inside backticks
- **THEN** no check reads it, and no rule asks for it to be reworded

## MODIFIED Requirements

### Requirement: The ledger is linted and neighbours are found without a model

`knowledge lint` SHALL report orphans, broken `[[links]]`, near-duplicate titles, stale
entries (superseded-by pending, hypothesis past its floor, promoted past its floor unless
`everyone`), and assumed kinds; it SHALL never edit. It SHALL run inside self-check.
`knowledge neighbours` SHALL rank entries by kind then keyword overlap; `capture --list`
SHALL group the pool by neighbourhood; `add` SHALL show the closest three live entries.

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

### Requirement: Knowledge is distributed to the agents who must know it

For audience `machine`, gtmux SHALL render one canonical file under the config dir and
maintain a sentinel-delimited managed block in each supported agent's global instruction
file carrying an INDEX (one entry per record + the canonical path), never the full text. In
that index the title SHALL stand on its own line, with the summary and the entry's id on the
line under it, and a leading copy of the entry's own slug SHALL be dropped from the title. For `repo`, gtmux SHALL maintain the same block, with full text, in the repository's
`AGENTS.md` (or `CLAUDE.md` when only that exists) and SHALL NOT commit. For `everyone`,
gtmux SHALL produce a prefilled issue URL and a copyable brief. Writes SHALL touch only
the inside of the block. `knowledge sync` SHALL refresh every carrier.

#### Scenario: A title that repeats the id it is filed under

- **WHEN** the machine index is rendered for `pitfalls/alias-makes-ops-noop`, whose title
  begins with that same slug
- **THEN** the title is printed without the slug, and the id appears under it beside the
  summary, so the entry can still be looked up

#### Scenario: A user hand-edits inside the block

- **WHEN** the block's content no longer matches its hash
- **THEN** `doctor` reports "knowledge sync: <agent> hand-edited", `sync` refuses to
  overwrite without `--force`, and nothing outside the block is ever touched

#### Scenario: An agent's instruction file does not exist

- **WHEN** the carrier file for a supported agent is absent
- **THEN** `sync` creates it containing only the block, and doctor shows it in sync
