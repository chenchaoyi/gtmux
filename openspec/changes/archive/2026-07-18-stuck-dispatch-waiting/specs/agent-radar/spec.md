# agent-radar — delta

## ADDED Requirements

### Requirement: A pre-turn startup gate or an unsubmitted dispatch draft reads as waiting

The radar SHALL treat two SCREEN states as `waiting` (needs-you) even though no hook
marker exists — a NARROW, explicit exception to "waiting is never inferred from screen
output", because these states BLOCK before any turn runs and fire no hook, so leaving
them `idle` would let them be reported as `done`:

- a session STARTUP/PERMISSION gate (e.g. Claude's "Do you trust the files in this
  folder?" confirmation, or another agent's equivalent), detected per-agent; and
- a STRUCTURED, non-empty input draft on a pane that is a TRACKED dispatch (the agent's
  goal was pasted but never submitted).

All OTHER waiting (tool-permission / plan / question) SHALL remain hook-driven and SHALL
NOT be inferred from the screen. The classification SHALL be pure (it MUST NOT write any
marker from the read path); the reclassified status carries a kind (`startup` / `draft`).

#### Scenario: A worker stuck at the trust gate reads as waiting

- **WHEN** a dispatched worker sits at its startup/trust gate (no hook has fired) and
  the radar would otherwise classify it `idle`
- **THEN** the radar reports it `waiting` (kind `startup`), so `gtmux tasks` / the digest
  never show it `done`

#### Scenario: A tracked dispatch with an unsubmitted draft reads as waiting

- **WHEN** a TRACKED dispatch pane holds a structured, non-empty input draft (the goal
  was pasted but the Enter was swallowed)
- **THEN** the radar reports it `waiting` (kind `draft`), not `idle`/`done`

#### Scenario: A normal idle pane is unaffected

- **WHEN** a pane is idle with an EMPTY input box, or holds a draft but is NOT a tracked
  dispatch (a human mid-compose)
- **THEN** it stays `idle` — the exception is scoped to startup gates + tracked-dispatch
  drafts only
