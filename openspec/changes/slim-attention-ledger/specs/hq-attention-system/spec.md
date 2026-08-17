# hq-attention-system (delta)

## REMOVED Requirements

### Requirement: Attention ledger

**Reason**: its write verbs (promote / re-prioritize / mark-surfaced / archive)
were never wired to any caller — HQ, the only intended writer, had no command
to perform them, so the tier/priority/surfaced fields and the archive were
forever empty. The live set stays small because `gtmux reap` removes settled
entries. The part that IS used — the disposition closed loop and `--verbose`
detail — lives in the pending-decision requirement below.

## MODIFIED Requirements

### Requirement: Pending-decision standing view

The ledger SHALL support a first-class "awaiting-commander" disposition, and
the system SHALL provide a standing view of exactly those entries (`gtmux tasks
--pending` — a filter on the existing ledger surface, no new top-level command), with
mark / unmark surfaces on the same command (`--await <id>` / `--resolve <id>
[disposition]`). The view SHALL be the single home of "what is on the commander's
plate": stable ordering, en+zh, and content that changes ONLY when an entry enters,
leaves, or changes state in the pending set. Briefs and reports reference this view
instead of re-printing the list (measured motivation: every brief of a 400+-turn shift
re-printed the same unchanged 「你手上未变: A · B · C…」 list — a persistent view
simulated in a scrolling transcript).

Each entry SHALL carry the free-text `disposition` and the `awaiting-since` /
first-seen / last-update timestamps — additive and optional, so a legacy entry
(including one written when the ledger carried tier/priority/surfaced/archive
fields) still loads. `gtmux tasks --verbose` SHALL add the disposition detail
to the live rows.

The ordering SHALL be TOTAL — the oldest wait first, then pane, then id — so
two reads of an unchanged set cannot differ. Every row renders at attention
grade (`▸`): an entry on the plate is a thing awaiting a decision, which is
not bookkeeping. The view
SHALL NOT render a relative clock (a countdown would churn the output the view
exists to stabilize) and SHALL NOT require a radar scan (it is polled, and the
plate is not a question about pane state).

Membership SHALL be the disposition and nothing else, so setting any other
disposition also removes an entry. The disposition is additive: a legacy
ledger entry without it still loads, none is pending, and marking one requires
no migration.

#### Scenario: The plate has one home

- **WHEN** an attention item is marked awaiting-commander
- **THEN** it appears in the pending view, and remains there unchanged-in-place until its
  state changes (decided, withdrawn, escalated)

#### Scenario: Two reads of an unchanged plate are identical

- **WHEN** the pending view is read twice with no entry entering, leaving, or changing
- **THEN** the two outputs are byte-identical

#### Scenario: Legacy entries are unaffected

- **WHEN** a ledger written before this change is loaded — with or without the
  retired tier/priority/surfaced/archive fields
- **THEN** every entry loads, none is in the pending set, and marking one
  awaiting-commander requires no migration
