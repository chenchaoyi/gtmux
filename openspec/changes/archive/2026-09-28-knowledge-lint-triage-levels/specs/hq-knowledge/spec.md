## MODIFIED Requirements

### Requirement: The ledger is linted and neighbours are found without a model

`knowledge lint` SHALL report orphans, broken `[[links]]`, near-duplicate entries, stale
entries (superseded-by pending, hypothesis past its floor, promoted past its floor unless
`everyone`), and assumed kinds; it SHALL never edit. It SHALL run inside self-check.
Its findings SHALL label unlinked standalone entries as `info` and similar-pair candidates
as `review`; neither label claims a defect. The self-check summary SHALL call unlinked
entries standalone, while the JSON `orphan` check name remains stable. Links are optional
for a valid entry. Wiki links in either language SHALL be checked and counted as
graph edges, with the language named in findings from an alternate body. Subjective
writing hints, unconfirmed kinds, missing translations, credential-shaped candidates,
and outdated references SHALL carry `review`; broken or ambiguous references and
other concrete maintenance failures SHALL carry `issue`.
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

#### Scenario: A standalone lesson has no wiki link

- **WHEN** a live knowledge entry has no incoming or outgoing wiki link
- **THEN** lint reports an `orphan` finding with `severity: info` and says links
  are optional; the self-check summary calls it a standalone entry

#### Scenario: Two lessons reuse words but answer different questions

- **WHEN** their text overlap crosses the similarity threshold
- **THEN** lint reports a `near-duplicate` finding with `severity: review` and
  asks HQ to compare facts, provenance, and scope before changing either entry

#### Scenario: The other language carries a wiki reference

- **WHEN** an entry's alternate-language body has a wiki link
- **THEN** lint checks that link for broken or ambiguous targets and counts a
  valid target as a graph edge, identifying the language in any finding
