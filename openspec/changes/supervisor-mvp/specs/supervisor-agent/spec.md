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

### Requirement: Human-in-the-loop boundary (P1)

The supervisor MUST NOT be granted automatic behaviors by gtmux in P1: gtmux
SHALL NOT auto-inject fleet events into the supervisor's pane, SHALL NOT let it
auto-answer other agents' permission prompts on the user's behalf, and ships no
orchestration (worktree spawning, cross-model dispatch). It reads and drives
through the same local CLI surface the user already has, when the user converses
with it. (Event nudging and orchestration are spec'd follow-ups, not P1.)

#### Scenario: No auto-drive without the user

- **WHEN** another agent enters waiting while the user is not conversing with
  the supervisor
- **THEN** gtmux delivers its normal notification pipeline only; nothing is
  injected into the supervisor pane
