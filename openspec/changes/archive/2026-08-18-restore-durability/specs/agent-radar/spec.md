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

BOTH states SHALL be judged from a COLOR-aware capture that EXCLUDES text the agent
renders FAINT (SGR 2) — its suggested-next-command ghost / placeholder, which needs a key
to accept and is therefore neither user input nor a question being asked. Only
normal-brightness content SHALL count, as a draft or as a gate.

A REOPENED-SESSION menu — the resume picker and its kin — SHALL be recognized, per-agent
through the same registry the gate check uses, and SHALL be classified as NEITHER. It is
not a draft: it is a numbered menu the agent PAINTED, and the draft detector, asked
whether the composer holds unsubmitted text, reads painted chrome as typed input (this
produced a `stuck before running — draft` alarm on 2026-08-18 about a session nothing had
been dispatched to — the third time in two weeks agent chrome has been misread as user
input). It is equally not a gate: a reopened session parked at its resume menu is not
blocked work, and calling it one recreates the "waiting forever" false positive the
picker exclusion exists to prevent. Picker recognition SHALL carry the same BOTTOM-REGION
and FAINT narrowings as gate recognition, so a pane merely quoting the menu's wording is
not sitting at one.

The exception SHALL be scoped to a dispatch that has NOT yet had its goal land. "Blocked
before any turn runs" is a claim about the DISPATCH, and the ledger already answers it: a
dispatch whose goal landed was accepted by the agent, so it is neither at a launch gate
nor holding that goal unsent. A dispatch that is delivered (or queued) SHALL NOT be
screen-classified at all. Neither SHALL a dispatch record so old that the pane id it
names has outlived it — see the session-restore capability's pane-record reconciliation.

A startup gate SHALL be recognized only in the BOTTOM REGION of the capture — chrome the
agent is currently DRAWING. A capture spans ~200 lines of scrollback, so a gate phrase
that merely APPEARS in it (a pane quoting or diffing the phrase) SHALL NOT read as a gate.

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

#### Scenario: A reopened-session menu is neither a draft nor a gate

- **WHEN** a tracked dispatch pane shows the agent's resume picker
- **THEN** the radar reads no draft and no gate — the menu is chrome the agent painted,
  and a session parked at it is not blocked work

#### Scenario: Menu wording in scrollback still leaves a real draft readable

- **WHEN** a pane's scrollback quotes the resume picker's wording while its composer
  holds a genuine unsubmitted draft
- **THEN** the radar still reports the draft — only the bottom region can be a menu

#### Scenario: A dim suggested-next-command is not a draft

- **WHEN** a tracked dispatch pane's composer shows only the agent's faint
  suggested-next-command ghost text (SGR 2), with no real user input
- **THEN** the radar does NOT read a draft and does NOT reclassify the pane as
  `waiting` — the ghost suggestion is excluded from draft detection

#### Scenario: A gate phrase the pane is QUOTING is not a gate

- **WHEN** a pane's scrollback contains a startup-gate phrase as CONTENT (a worker
  reading, diffing or discussing it) while the pane itself shows an ordinary composer
- **THEN** the radar does NOT read a startup gate — only the bottom region, where the
  agent draws its own chrome, is considered

#### Scenario: A dispatch that already ran is never screen-classified

- **WHEN** a tracked dispatch whose goal has LANDED is idle
- **THEN** the radar does NOT apply the stuck-before-running exception to it at all —
  neither `startup` nor `draft` — because a landed goal proves the pane is past both

#### Scenario: A normal idle pane is unaffected

- **WHEN** a pane is idle with an EMPTY input box, or holds a draft but is NOT a tracked
  dispatch (a human mid-compose)
- **THEN** it stays `idle` — the exception is scoped to startup gates + tracked-dispatch
  drafts only
