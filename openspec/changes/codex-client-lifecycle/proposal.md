# Codex client lifecycle and ownership evidence

## Why
Desktop can view CLI-created conversations and continue them after writer release.
gtmux uses creation originator as a client label and can retain an early provisional
native record after verified tmux binding; it resurfaces after reap. Stored history,
client liveness, storage writer, input controller, turn state and HQ authority need
separate treatment.

## What Changes
- Retire obsolete provisional observations with exact binding evidence, protected
  against newer independent activity; correlate reap receipts and old cases.
- Separate thread identity, creation source and verified input route in the Go core;
  preserve fork, server and Codex-home distinctions and wire compatibility.
- Make counts, menus, HQ follow/knowledge and notifications consume current evidence,
  with explicit unknown control and existing consent boundaries.
- Plan a versioned read-only history/status adapter without resume side effects.
- Keep transfer and existing-client inline restart behind separate proven operations
  and explicit real-client decisions; no general handoff is claimed.

## Impact
Proposed deltas: native-agent-sessions, agent-radar, chat-transcript. CLI, menubar,
web, iPhone and iPad consume the same core evidence. A is merged; B/C implement conservative identity, consent and stored-history reading.
Current capability specs track those changes; unresolved frontend ownership, lineage
and handoff remain proposed.
Design/evidence: docs/design/codex-client-lifecycle.md and its Chinese twin.

## Surfaces
- CLI / terminal: structured identity, ownership evidence and lifecycle receipts; input stays on the verified pane route.
- menubar: current-client labels, exact pane focus, read-only conversation and gated local Desktop opening.
- web: the same counts/capabilities; guests remain restricted to granted tmux panes.
- iPhone / phone: paired-host reading and verified pane actions; no Mac-local deep link on iOS.
- iPad: the same core policy in its workspace, with separate real-device layout/accessibility acceptance.

## Boundaries
No real resume/turn/fork/rotation, shutdown, archive/delete, Codex lock/SQLite mutation,
credential/global configuration changes, release or install. Actual IDs/logs stay
private. Source tests were inspected, not executed. Real handoff and device accessibility
remain unverified. Remaining tasks are explicitly unchecked; completed implementation is recorded separately.
