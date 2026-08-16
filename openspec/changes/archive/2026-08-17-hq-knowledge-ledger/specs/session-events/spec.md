# session-events (delta)

## MODIFIED Requirements

### Requirement: The journal carries gtmux's own control and audit records

gtmux's own records SHALL ride the same journal and `Record` shape as agent
lifecycle events, namespaced apart from them by the `gtmux:` event prefix, and
SHALL be split into two kinds with different debt semantics:

- **Control records** (`gtmux:` outside the audit sub-namespace — maintenance
  triggers, degradations, reconciles) carry NEW information for the supervisor.
  They SHALL count toward the consumption debt, since the `unread` knock is the
  only channel a pane-less record has.
- **Audit records** (the `gtmux:audit:` sub-namespace) document an act whose
  actor already knows it — a delivered or dropped wake, a supervisor-driven send,
  a reap, a rotation, a knowledge-ledger operation. They are TRAIL, not debt:
  they SHALL NOT count toward the consumption debt, SHALL be omitted from the
  supervisor's default delta pull exactly as the tally omits them, and SHALL
  remain in the log — returned by `--all` and by any non-supervisor read.

Every audit record SHALL nest inside the control namespace, so the standing rule
that sensors exclude control records from the deltas they measure covers audit
records with no further case analysis. An audit record's severity SHALL be
`routine` — it never asks for attention.

Audit summaries SHALL be bounded and single-line: a payload longer than its
record's budget SHALL truncate at a rune boundary, and embedded newlines SHALL
collapse, so one record is always one journal line.

#### Scenario: An audit record is a control record

- **WHEN** a `gtmux:audit:wake-delivered` record is appended
- **THEN** it is classified as a control record (sensor deltas exclude it), renders
  through the control formatting of `gtmux events`, and carries routine severity

#### Scenario: An audit record never becomes debt

- **WHEN** the only records past the supervisor's watermark are `gtmux:audit:*`
- **THEN** no `unread` wake is raised for them, and they are still returned by
  `gtmux events --since-seq <n> --all` and by any non-supervisor read

#### Scenario: A maintenance trigger still becomes debt

- **WHEN** a `gtmux:distill` control record (not audit) lands past the watermark
- **THEN** it counts toward the consumption debt exactly as before

#### Scenario: An oversized audit payload stays one bounded line

- **WHEN** an audit record is constructed from a multi-line payload longer than its
  budget
- **THEN** the stored summary is single-line, truncated at a rune boundary, and the
  journal line parses as one record
