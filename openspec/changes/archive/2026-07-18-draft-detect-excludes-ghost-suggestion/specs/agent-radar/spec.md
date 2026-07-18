# agent-radar (delta)

## MODIFIED Requirements

### Requirement: A pre-turn startup gate or an unsubmitted dispatch draft reads as waiting

The radar SHALL treat two SCREEN states as `waiting` (needs-you) even though no hook
marker exists — a NARROW, explicit exception to "waiting is never inferred from screen
output", because these states BLOCK before any turn runs and fire no hook, so leaving
them `idle` would let them be reported as `done`:

- a session STARTUP/PERMISSION gate (e.g. Claude's "Do you trust the files in this
  folder?" confirmation, or another agent's equivalent), detected per-agent; and
- a STRUCTURED, non-empty input draft on a pane that is a TRACKED dispatch (the agent's
  goal was pasted but never submitted).

The draft state SHALL be judged from a COLOR-aware capture that EXCLUDES the agent's
suggested-next-command GHOST text — the dim autosuggestion the agent renders faint
(SGR 2) and that needs a key to accept — because it is NOT user input: only genuinely
unsubmitted USER input (normal brightness) SHALL count as a draft. All OTHER waiting
(tool-permission / plan / question) SHALL remain hook-driven and SHALL NOT be inferred
from the screen. The classification SHALL be pure (it MUST NOT write any marker from
the read path); the reclassified status carries a kind (`startup` / `draft`).

#### Scenario: A worker stuck at the trust gate reads as waiting

- **WHEN** a dispatched worker sits at its startup/trust gate (no hook has fired) and
  the radar would otherwise classify it `idle`
- **THEN** the radar reports it `waiting` (kind `startup`), so `gtmux tasks` / the digest
  never show it `done`

#### Scenario: A tracked dispatch with an unsubmitted draft reads as waiting

- **WHEN** a TRACKED dispatch pane holds a structured, non-empty input draft (the goal
  was pasted but the Enter was swallowed)
- **THEN** the radar reports it `waiting` (kind `draft`), not `idle`/`done`

#### Scenario: A dim suggested-next-command is not a draft

- **WHEN** a tracked dispatch pane's composer shows only the agent's faint
  suggested-next-command ghost text (SGR 2), with no real user input
- **THEN** the radar does NOT read a draft and does NOT reclassify the pane as
  `waiting` — the ghost suggestion is excluded from draft detection

#### Scenario: A normal idle pane is unaffected

- **WHEN** a pane is idle with an EMPTY input box, or holds a draft but is NOT a tracked
  dispatch (a human mid-compose)
- **THEN** it stays `idle` — the exception is scoped to startup gates + tracked-dispatch
  drafts only
