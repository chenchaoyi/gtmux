# Supervisor Agent Specification

## ADDED Requirements

### Requirement: Launchable supervisor session

The system SHALL provide `gtmux hq` (中控): it creates — or focuses, when one
already runs — a dedicated tmux session running the user's coding agent (Claude
by default, per existing agent profiles) with its working directory set to the
persistent supervisor home (`~/.config/gtmux/hq/`). On first run the home SHALL
be seeded with a generated instructions file (CLAUDE.md) teaching the supervisor
loop — read `gtmux digest --json`, judge, drill into a pane
(`tmux capture-pane`) only when warranted, drive via `gtmux send`, report to the
user — and SHALL never be overwritten once present, so the user can edit it and
the supervisor's accumulated knowledge persists across sessions.

#### Scenario: First launch seeds the home

- **WHEN** `gtmux hq` runs and `~/.config/gtmux/hq/` has no instructions file
- **THEN** the home is created, the instructions file is generated, and a tmux
  session starts the agent there

#### Scenario: Relaunch reuses, never clobbers

- **WHEN** `gtmux hq` runs while a supervisor session is already live
- **THEN** it focuses the existing session instead of spawning a second, and an
  existing (possibly user-edited) instructions file is left untouched

### Requirement: Supervisor visibility in the radar

A supervisor session SHALL appear in the radar like any agent, additionally
marked with an optional `role:"supervisor"` field in `agents --json` — detected
by its pane cwd being the supervisor home (robust to session renames) — so
surfaces can pin or badge it. The field is additive and absent for normal
agents.

#### Scenario: Supervisor row is marked

- **WHEN** the supervisor session is live and `gtmux agents --json` runs
- **THEN** its row carries `role:"supervisor"`; all other rows are unchanged

### Requirement: Waiting-event nudge into the supervisor

The system SHALL, when a tmux agent enters waiting and a supervisor session is
live, inject ONE compact line — the location, waiting kind, and title — into the
supervisor's pane (send-keys + Enter), riding the notification pipeline's
existing dedup so an unchanged waiting state is not re-nudged. It SHALL never
nudge the supervisor about its own waiting states, SHALL be a no-op when no
supervisor session is live, and SHALL be disableable via configuration
(`hqNudge: false`, default on).

#### Scenario: Agent blocks, supervisor learns

- **WHEN** another agent enters waiting (permission/plan/question) while an hq
  session is live
- **THEN** one `[gtmux] waiting·<kind> <loc> — <title>` line is typed into the
  hq pane, at most once per waiting transition

#### Scenario: Never about itself, off when absent or disabled

- **WHEN** the supervisor itself is the waiting pane, or no hq session is live,
  or `hqNudge` is false
- **THEN** nothing is injected

### Requirement: Human-in-the-loop boundary (P1)

Beyond the nudge (inform-only), the supervisor MUST NOT be granted automatic
behaviors by gtmux in P1: gtmux SHALL NOT let it auto-answer other agents'
permission prompts on the user's behalf, and ships no orchestration (worktree
spawning, cross-model dispatch). What the supervisor DOES upon a nudge is
governed by its editable instructions file, whose generated default is assess +
report — driving stays a conversational act.

#### Scenario: Nudge informs, does not answer

- **WHEN** a nudge lands for another agent's permission prompt
- **THEN** gtmux itself sends nothing to the WAITING pane; any follow-up action
  is the supervisor's turn under its instructions
