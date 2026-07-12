# session-events Specification

## Purpose
TBD - created by archiving change session-events. Update Purpose after archive.
## Requirements
### Requirement: Append-only session event log

The system SHALL append one JSON record per agent lifecycle event — for every
session, tmux or native — to a bounded log at `~/.local/share/gtmux/events.jsonl`,
fed by the SAME hook that writes the state markers and the notify queue (additive;
those are unchanged). Each record SHALL carry at least a timestamp, the event, the
derived state, and the session's identity (pane/loc/session/agent) plus the
waiting kind when applicable. The log SHALL ROTATE at a size cap — the active file
renamed to `events.1.jsonl` (overwriting any prior) and a fresh one started,
keeping one rotated generation — so total on-disk size is bounded (active + 1
rotated; default 20 MB cap → ≈ 40 MB ceiling) and it can never single-point-explode.

#### Scenario: Every event is logged

- **WHEN** the hook fires for any session (start/stop/waiting/…)
- **THEN** a JSON line for it is appended to events.jsonl, with ts/event/state/
  identity, without altering the existing markers or notify queue

#### Scenario: The log rotates and stays bounded

- **WHEN** the active log exceeds its size cap
- **THEN** it is rotated to a numbered generation and a fresh file starts; old
  generations beyond the retained count are removed, so total size stays bounded

#### Scenario: Follow survives rotation

- **WHEN** `gtmux events --follow` is running and the log rotates
- **THEN** the follower re-opens the new file and keeps emitting new events (it
  does not silently stop)

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

