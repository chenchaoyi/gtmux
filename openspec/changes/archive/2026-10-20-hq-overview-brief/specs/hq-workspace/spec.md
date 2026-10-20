## ADDED Requirements

### Requirement: Overview is a bounded brief with explicit provenance
The workspace list SHALL accept an opt-in `--overview` projection built from the same loaded database. Ordinary list output SHALL omit assessment bodies. Overview SHALL show at most three needs-input cards, three following cards and five recent changed matters. Cards SHALL preserve the latest HQ message ID, kind and recorded clock with at most 768 Unicode characters and a truncation flag. A fallback saved assessment SHALL NOT inherit the matter update time as its report time or wait start. Reading SHALL NOT invoke a model, parse event logs/transcripts, mutate records or consume HQ's event watermark.

Current worker status SHALL require exact agent/home/session identity and an ongoing execution association. Idle or a result report SHALL NOT complete a matter. Full-category selection and card selection SHALL open original matter scope without sending an instruction. Wide layouts SHALL place the two brief categories side by side and narrow layouts SHALL stack them. The full sidebar directory and compact handoff entry SHALL remain available.

Recent changes SHALL use explicit state transitions, results, decisions, errors, unconfirmed delivery, registration, merges and splits within the recorded last 24 hours. They SHALL omit ordinary progress, delivery receipts and supervisor wake/turn noise, deduplicate by canonical matter and retain a count of changed matters rather than raw events. Missing clocks, future records and broken identity edges SHALL NOT create recent entries.

#### Scenario: A quiet worker still has unfinished work
- **WHEN** an exact linked execution conversation is idle but its matter remains open
- **THEN** the card reports the current conversation state without completing or reclassifying the matter

#### Scenario: Long history and reports
- **WHEN** the database has many ended matters and long multilingual reports
- **THEN** the default brief remains bounded, excerpts preserve Unicode and original source metadata, and View all keeps the full explicit category reachable

#### Scenario: No report clock exists
- **WHEN** a matter has only an assessment without its source message
- **THEN** the note is labelled as a saved assessment with no invented clock or wait duration
