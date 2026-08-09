# hq-attention-system (delta)

## ADDED Requirements

### Requirement: Pending-decision standing view

The attention ledger SHALL support a first-class "awaiting-commander" disposition, and
the system SHALL provide a standing view of exactly those entries (a filter on the
existing ledger surface, e.g. `gtmux tasks --pending` — no new top-level command). The
view SHALL be the single home of "what is on the commander's plate": stable ordering,
en+zh, and content that changes ONLY when an entry enters, leaves, or changes state in
the pending set. Briefs and reports reference this view instead of re-printing the list
(measured motivation: every brief of a 400+-turn shift re-printed the same unchanged
「你手上未变: A · B · C…」 list — a persistent view simulated in a scrolling transcript).

The disposition is additive: a legacy ledger entry without it still loads, and archiving
semantics are unchanged.

#### Scenario: The plate has one home

- **WHEN** an attention item is marked awaiting-commander
- **THEN** it appears in the pending view, and remains there unchanged-in-place until its
  state changes (decided, withdrawn, escalated)

#### Scenario: Legacy entries are unaffected

- **WHEN** a ledger written before this change is loaded
- **THEN** every entry loads, none is in the pending set, and marking one
  awaiting-commander requires no migration

### Requirement: Attention grades render distinctly on gtmux-owned surfaces

Surfaces gtmux itself renders — at minimum `gtmux events --follow` and the tasks/ledger
views — SHALL render each row's attention grade (decision · attention · ledger, the same
three-value scale the wake lines carry) distinctly: in color when stdout is a tty,
suppressed under `NO_COLOR`, and byte-identical to today's output when not a tty (pipes
and scripts see no change). Color encodes ONLY the grade — consistent with the design
rule that color carries exactly one meaning per surface. The agent-facing pane content
is out of scope here: the composer and agent TUIs cannot carry ANSI color, which is why
the wake line's grade is a glyph (see hq-wake-protocol).

#### Scenario: A tty read scans by grade

- **WHEN** `gtmux events --follow` runs on an interactive terminal
- **THEN** decision-grade rows are visually distinct from attention- and ledger-grade
  rows by color, and the glyph/text content is identical to the uncolored form

#### Scenario: Scripts see no change

- **WHEN** the same command's output is piped, or `NO_COLOR` is set
- **THEN** the bytes carry no escape sequences and match today's format
