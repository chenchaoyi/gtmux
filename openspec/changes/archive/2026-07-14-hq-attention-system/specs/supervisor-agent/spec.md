## ADDED Requirements

### Requirement: HQ subscribes to the silent feed and gates its own output

The seeded playbook SHALL teach HQ to consume the full event stream through the silent
perception feed (a background subscription to `gtmux hq-feed`) and to GATE its own
user-visible output by surfacing tier: it SHALL print to the user for CRITICAL and NORMAL
items (per the resolved `surfaceTier`), and for QUIET items it SHALL only record to the
attention ledger and stay silent that turn. HQ SHALL answer confirm-type asks itself only
within the reversible ∧ low-risk ∧ no-fork bound (recording the auto-answer), and escalate
everything else. HQ SHALL always surface a feed-degradation CRITICAL regardless of the
configured threshold.

#### Scenario: A QUIET event produces no user output

- **WHEN** HQ receives a QUIET-tier event through the feed
- **THEN** it records the item in the ledger and prints nothing to the user that turn

#### Scenario: A CRITICAL event is surfaced

- **WHEN** HQ receives a CRITICAL-tier event (e.g. a decision-type ask, an error, or a
  feed degradation)
- **THEN** HQ prints it to the user, and a feed-degradation CRITICAL is surfaced even
  when quiet mode is on

#### Scenario: The threshold moves the bar, not HQ's awareness

- **WHEN** the user raises the surfacing threshold (quiet on)
- **THEN** HQ still ingests every event but prints only CRITICAL items, the rest going to
  the ledger

### Requirement: HQ self-check and self-maintenance

The seeded playbook SHALL teach HQ, on a gtmux-raised self-check trigger, to review and
maintain its OWN artifacts — event-log/feed health, attention-ledger archival and
de-duplication, memory/knowledge-base quality, and accumulated low-value items — using only
its existing write-own-notes authority. HQ SHALL default to SILENT self-maintenance,
printing a one-line brief ONLY when it took a real action, and SHALL escalate a severe
finding (rotation broken, cursor gap, mass-invalid memory) as CRITICAL.

#### Scenario: Silent maintenance when nothing needed

- **WHEN** a self-check trigger fires and HQ finds nothing to fix
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real cleanup is briefed in one line

- **WHEN** a self-check trigger fires and HQ archives closed ledger items or prunes stale
  memory
- **THEN** HQ prints a single one-line brief of what it did

#### Scenario: A severe finding escalates

- **WHEN** a self-check finds a broken rotation, a cursor gap, or mass-invalid memory
- **THEN** HQ surfaces it as CRITICAL rather than quietly cleaning up
