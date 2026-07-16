# supervisor-agent — delta

## MODIFIED Requirements

### Requirement: HQ subscribes to the silent feed and gates its own output

The seeded playbook SHALL teach HQ to perceive by PULL-ON-WAKE: on any wake line
it reads the delta (`gtmux events --since <seq>` and/or `gtmux digest --json`)
before acting, rather than requiring a persistently backgrounded
`gtmux hq-feed --tail` subscription (which is agent-specific and is DROPPED as a
playbook requirement — the spool remains available as pull-side data). HQ SHALL
GATE its own user-visible output by surfacing tier: it SHALL print for CRITICAL
and NORMAL items (per the resolved threshold), and for QUIET items it SHALL only
record to the attention ledger and stay silent that turn. HQ SHALL answer
confirm-type asks itself only within the reversible ∧ low-risk ∧ no-fork bound
(recording the auto-answer), and escalate everything else. HQ SHALL always
surface a feed-degradation CRITICAL regardless of the configured threshold.

#### Scenario: Wake then pull, on any agent

- **WHEN** HQ (running on any CLI agent, Claude or not) receives a wake line
  covering seq 341-352
- **THEN** it pulls the delta via CLI commands before acting — no background
  subscription is assumed

#### Scenario: A QUIET event produces no user output

- **WHEN** HQ ingests a QUIET-tier event from a pulled delta
- **THEN** it records the item in the ledger and prints nothing to the user that turn

#### Scenario: A CRITICAL event is surfaced

- **WHEN** HQ ingests a CRITICAL-tier event (a decision-type ask, a crash, or a
  feed degradation)
- **THEN** HQ prints it, and a feed-degradation CRITICAL is surfaced even when quiet
  mode is on

## ADDED Requirements

### Requirement: Enrollment — goal-aware dossiers for every sensed session

On HQ start the seeded playbook SHALL direct a fleet enrollment: read the full
digest and record a dossier per agent session on the situation board — purpose
(the session's goal), current status, and channel (hq-dispatched / user-direct) —
drilling into a transcript head at most once per session only when the purpose is
not evident from the digest. Thereafter each `new-session` wake SHALL enroll the
newcomer incrementally. Perception SHALL remain goal-aware: board entries name
what a session is FOR, not merely its mechanical state.

#### Scenario: HQ start builds the fleet dossier

- **WHEN** HQ starts with nine live agent sessions
- **THEN** its first turns produce a board with nine dossiers (purpose / status /
  channel), with at most one transcript-head drill per unclear session

#### Scenario: A newcomer is enrolled incrementally

- **WHEN** a new agent session appears while HQ is live
- **THEN** HQ receives one `new-session` wake and appends that session's dossier to
  the board without re-scanning the fleet

### Requirement: Signal register separates wake traffic from conversation

The seeded playbook SHALL mandate two output registers: replies to wake lines use
the SIGNAL register — one line opening with `⟣` and a fixed glyph vocabulary
(✅ done-judgment, ▪ noted-to-board, ◈ tick brief, ⚠ escalation), with at most
two indented detail lines (tick briefs ≤6 lines total) — while replies to the
human use ordinary conversational prose with no sigils. Wake turns SHALL be
short: pull the delta, judge, update the board, emit the signal line; no
narration.

#### Scenario: A done wake gets a one-line judgment

- **WHEN** HQ processes a `done` wake for a session that completed its goal
- **THEN** its reply is a single `⟣ ✅ …` line (judgment + suggested next step),
  visually distinct from conversation

#### Scenario: A trivial done is noted silently

- **WHEN** HQ judges a done wake to be an unremarkable intermediate completion
- **THEN** it updates the board and replies with at most one `⟣ ▪` note line

### Requirement: Periodic tick brief

On a `tick` wake the playbook SHALL direct HQ to pull the covered delta, update
the situation board, and emit ONE brief in the signal register — at most six
lines: a `⟣ ◈` headline with fleet counts and the top item needing attention,
then up to five indented outcome lines (completions with a one-clause summary,
new sessions, stalls). The brief SHALL respect the resolved quiet threshold; in
quiet mode board-only unless something CRITICAL rode the tick.

#### Scenario: The brief is bounded and concrete

- **WHEN** a tick wake covers three completions and one new session
- **THEN** HQ emits one ≤6-line `⟣ ◈` brief naming each outcome in one clause, and
  nothing else

### Requirement: Managed playbook migrates legacy homes

`gtmux hq` SHALL migrate a legacy HQ home whose only policy file is a full
CLAUDE.md (no managed AGENTS.md): back up the legacy file alongside
(timestamped, never deleted), generate the managed AGENTS.md at the current
playbook version plus the `@AGENTS.md` CLAUDE.md pointer, seed LOCAL.md once,
and print a one-line notice naming the backup. A home with a managed AGENTS.md
SHALL continue on the existing upgrade path. The seeder SHALL NOT silently skip
any home shape.

#### Scenario: A legacy home is upgraded on next start

- **WHEN** `gtmux hq` runs against a home containing only a legacy full CLAUDE.md
- **THEN** the legacy file is backed up in place, the managed AGENTS.md + pointer +
  LOCAL.md are written at the shipped version, and the notice names the backup
