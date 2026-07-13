# agent-dispatch Specification

## MODIFIED Requirements

### Requirement: Dispatch ledger and needs-you view

The system SHALL record each `gtmux spawn` dispatch (task id → pane → goal → model
→ status), INCLUDING what the dispatch created (its session/window, worktree path,
and branch, for later reclamation) AND an additive `source` field
(`hq-dispatched` | `user-direct` | `agent-self`), and expose `gtmux tasks [--json]`.
`gtmux spawn` SHALL stamp `source: "hq-dispatched"`; `user-direct`/`agent-self`
entries are ones HQ back-fills from work it sensed (gtmux does not fabricate them).
A ledger entry's lifecycle status (delivered → working → waiting → done) SHALL be
derived from the dispatched pane's live radar state. `gtmux tasks` SHALL lead with
entries needing attention (a tracked pane that is waiting or done-after-work), the
same needs-you-first ordering the digest uses. The `source` field is additive and
optional — an entry without it is treated as `hq-dispatched`.

#### Scenario: A dispatch is tracked

- **WHEN** `gtmux spawn <goal>` succeeds
- **THEN** a ledger entry exists for it with `source: "hq-dispatched"` and
  `gtmux tasks` lists it with its live status

#### Scenario: Needs-you ordering

- **WHEN** `gtmux tasks` runs and a tracked pane is waiting or done-after-work
- **THEN** that entry is listed ahead of still-working ones

#### Scenario: Source round-trips

- **WHEN** a ledger entry is written with a `source` and read back
- **THEN** the same source is returned; a legacy entry without one reads as
  `hq-dispatched`
