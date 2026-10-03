# agent-radar (delta)

## ADDED Requirements

### Requirement: Codex idle chrome cannot create a completed turn

The radar SHALL keep a Codex pane idle when only a warning banner changes above
its ready composer and no active or waiting turn marker exists. It SHALL keep
real visible work and hook-marked turns eligible for working and completion.
The pane SHALL NOT borrow another agent's resume record for transcript or
completion evidence. A unique current Codex rollout with `session_meta.id`
SHALL be eligible for cwd-based session binding.

#### Scenario: Weekly usage warning before the first turn

- **WHEN** an idle Codex pane with an old Claude resume record redraws its
  weekly usage warning
- **THEN** radar stays idle and no false completion alert is produced

#### Scenario: Work visibly begins

- **WHEN** the same Codex pane displays a live working or tool status
- **THEN** radar may report working and later report a real completion
