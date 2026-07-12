# Session Events Specification

## ADDED Requirements

### Requirement: Append-only session event log

The system SHALL append one JSON record per agent lifecycle event — for every
session, tmux or native — to a bounded log at `~/.local/share/gtmux/events.jsonl`,
fed by the SAME hook that writes the state markers and the notify queue (additive;
those are unchanged). Each record SHALL carry at least a timestamp, the event, the
derived state, and the session's identity (pane/loc/session/agent) plus the
waiting kind when applicable. The log SHALL be size-bounded (older lines dropped
past a cap) so it never grows without limit.

#### Scenario: Every event is logged

- **WHEN** the hook fires for any session (start/stop/waiting/…)
- **THEN** a JSON line for it is appended to events.jsonl, with ts/event/state/
  identity, without altering the existing markers or notify queue

#### Scenario: The log is bounded

- **WHEN** the log exceeds its size cap
- **THEN** the oldest lines are dropped so it stays within the cap

### Requirement: Events reader and subscription

The system SHALL provide `gtmux events [--follow] [--json] [--since <dur>]` to
read the recent stream or `--follow` it live — the terminal-native subscription
to all sessions' execution, usable by gtmux HQ and any script.

#### Scenario: Tail the live stream

- **WHEN** a consumer runs `gtmux events --follow`
- **THEN** it receives existing recent events and then each new event as it is
  appended, until interrupted

#### Scenario: Recent window

- **WHEN** `gtmux events --since 10m` is run
- **THEN** only events from the last 10 minutes are printed
