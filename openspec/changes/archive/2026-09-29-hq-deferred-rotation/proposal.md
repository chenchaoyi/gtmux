# HQ deferred rotation

## Why

An active Codex HQ turn can run `gtmux hq --rotate` while its composer appears empty.
The old command pasted `/new`, reported that rotation was underway, and logged a
successful rotate act. Codex rejected it with `'/new' is disabled while a task is in
progress.` No successor `SessionStart` followed. A visible composer is therefore not
proof that the current task has ended, and a successful tmux key send is not proof of
session rotation.

## Design

The command records one durable request bound to the live HQ pane, agent, and retiring
session ID. It reports **queued**. The resident slow tick waits for same-session turn
completion evidence, an empty input box, and a short UI settling interval. It then
pastes and submits the agent's reset once. The attempt is marked sent before either
key operation so a crash cannot blindly submit a second reset over a possible draft. A distinct new
session ID confirms completion; failure or timeout creates an explicit audit record.
The existing HQ board and knowledge base carry the handoff across the reset.

The conservative edge is deliberate: missing completion evidence, an unreadable
composer, a draft, copy mode, or a changed pane/session prevents delivery. Waiting
expires after 30 minutes; a sent command with no successor expires after 45 seconds.
The queue survives a serve restart. A later manual request may retry after a failure,
but the service does not retry an uncertain send on its own.

## Surfaces

- **Terminal / remote attach**: `gtmux hq --rotate` reports queued; `gtmux events --all`
  exposes requested, confirmed, and failed receipts with session IDs.
- **Menu bar**: no separate rotation control; it consumes the same HQ state.
- **Phone**: no separate rotation control; the existing HQ view continues to use the
  shared state and successor identity.
- **iPad**: same shared HQ view as phone; no new control.
- **Web**: no HQ rotation control; no UI change.

## Limits

The service cannot make Codex's slash command atomic with a concurrent user keystroke.
It uses the shared two-frame composer guard and rechecks the task boundary immediately
before typing. It cannot prove what happened if the TUI accepted a reset but session
identity was not subsequently observable; that outcome is recorded as failed rather
than successful. No live HQ rotation is part of this change's test procedure.
