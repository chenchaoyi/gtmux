## ADDED Requirements

### Requirement: Candidate settlement preserves evidence and pending work

Every newly recorded candidate SHALL have a stable identity and a digest of its retained
payload. Acceptance and dismissal SHALL append a ledger operation carrying every selected
candidate's original retained text, context and available metadata, with a disposition,
operation ID and dismissal reason where applicable. Unknown source positions SHALL remain
unknown. Legacy candidates SHALL receive deterministic read-time identities without
rewriting their content.

Selection and settlement SHALL serialize across processes. A candidate SHALL leave the
pending view only after a ledger operation settles its identity. A failure before commit
SHALL preserve both the prior ledger and pending work. A later candidate sharing a family
key SHALL remain pending. Dismissals SHALL NOT create live knowledge entries.

#### Scenario: Validation or write fails after selecting candidates
- **WHEN** acceptance cannot commit its ledger operation
- **THEN** all selected candidates remain pending, their original evidence is readable,
  and the command reports a correlated uncommitted failure

#### Scenario: The commit succeeds but rendering fails
- **WHEN** a candidate settlement is committed and its derived view cannot refresh
- **THEN** the receipt and sources remain committed, the candidate is settled exactly once,
  and the error names the committed operation and the repair command

#### Scenario: A source family recurs after settlement
- **WHEN** a new candidate has the key of an already settled candidate
- **THEN** only the previously settled identity is excluded from the pending view

### Requirement: Source evidence is available without automatic distribution

Entry detail reads and `knowledge show --json` SHALL expose retained source snapshots.
`knowledge receipts [--capture <key>] [--json]` SHALL read committed accepted/dismissed
settlements, including operation IDs, reasons and sources, from any directory. Supersede
SHALL preserve source lineage. Source excerpts SHALL NOT appear in the index, generated
carriers or public promotion briefs. Existing sensitive-entry restrictions SHALL hold.

#### Scenario: Reviewing a dismissed line
- **WHEN** a reader asks for receipts for its capture key
- **THEN** the original retained candidate and dismissal reason are available even after
  the pending view no longer lists it
