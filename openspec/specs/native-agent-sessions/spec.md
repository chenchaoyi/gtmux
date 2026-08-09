# native-agent-sessions Specification

## Purpose
TBD - created by archiving change native-agent-sessions. Update Purpose after archive.
## Requirements
### Requirement: Sense agent sessions running outside tmux
The system SHALL record the existence and state of an agent session that invokes `gtmux hook` while running outside tmux (no `$TMUX_PANE`), keyed by the agent's `session_id` rather than a tmux pane id. The record SHALL capture at least `{agent, sessionId, cwd, state, updatedAt}` — plus the agent process's pid and command name, used by the liveness reap below, and the hosting terminal app's display name ("Warp", "Ghostty", …) sensed best-effort from the hook's own environment/ancestry ("" when unrecognized) — where `state` is derived from the SAME hook lifecycle (`decide()`) used for tmux panes. The system SHALL NOT record an agent's internal warm-spare/pool process (e.g. Claude's `bg-spare`), which fires a hook but is never a real user-facing session. The system SHALL likewise NOT sense an agent's internal HELPER call (e.g. Codex's ambient-suggestions generator or auto-mode safety classifier), which fires a full pane-less hook lifecycle under a session id that is nobody's conversation: a pane-less `UserPromptSubmit` whose prompt head matches a known helper system prompt SHALL remove the session's native record, mark the session id, and swallow the session's later events — including from the event stream and the supervisor's unread debt (the already-streamed `SessionStart` is paired with a `SessionEnd` so the pane-less lifecycle-blink exclusion covers it). Detection SHALL rest on that positive prompt evidence, never on the empty pane alone (a real native session is pane-less too).

#### Scenario: Hook fires with no tmux pane
- **WHEN** `gtmux hook` runs with an empty `$TMUX_PANE` and a payload carrying `session_id` and `cwd`
- **THEN** the system SHALL write/update a native-session record keyed by `session_id` (instead of degrading to a stateless notify) reflecting the event's derived state

#### Scenario: Warm-spare process is not sensed
- **WHEN** a hook fires (no `$TMUX_PANE`, with a `session_id`) but the agent process is an internal warm-spare (its command name is `bg-spare`)
- **THEN** the system SHALL NOT create a native record for it

#### Scenario: Internal helper call is filtered at the prompt
- **WHEN** a pane-less `UserPromptSubmit` fires whose prompt head matches a known agent-internal helper system prompt (e.g. Codex's ambient-suggestions generator)
- **THEN** the system SHALL remove any native record the session's `SessionStart` created, SHALL not stream the event, and SHALL swallow the session's subsequent events

#### Scenario: A real native session is not mistaken for a helper
- **WHEN** a pane-less `UserPromptSubmit` fires with an ordinary (non-helper) prompt
- **THEN** the session SHALL be sensed and tracked exactly as before — the empty pane alone is never the filter

#### Scenario: Lifecycle transitions update state
- **WHEN** successive hooks fire for the same `session_id` (e.g. UserPromptSubmit then Stop)
- **THEN** the record's `state` SHALL move working → idle following the same transitions as a tmux-keyed session, and its idle "finished" time SHALL be derivable session-independently of any tmux window activity

### Requirement: Native sessions appear in the radar as source "native"
`gtmux agents --json` SHALL include native sessions as rows with `source: "native"`, carrying agent, project (cwd), state, an idle "finished N ago" time, and the sensed hosting terminal name in the `terminal` field (omitted when unrecognized). These rows SHALL omit any focusable tmux locator and SHALL be marked as neither focusable nor send-able. A native session whose `session_id` also corresponds to a live tmux pane SHALL NOT be double-listed (the tmux row wins). A native row SHALL be listed only on positive evidence that something real is behind it — its record names a live process, or its session has an on-disk conversation; a record with neither (an unidentified helper call's residue) SHALL be withheld from every surface rather than shown as a convincing fake.

#### Scenario: Native session listed alongside tmux ones
- **WHEN** a native session has a current record and no matching live tmux pane
- **THEN** `agents --json` SHALL include one row for it with `source: "native"` and no focusable locator

#### Scenario: Row names the hosting terminal
- **WHEN** a native session's hook fired from a recognized terminal app (e.g. a plain Warp window)
- **THEN** its radar row SHALL carry `terminal` with that app's display name (e.g. "Warp"), so the surfaces can label where the out-of-tmux agent lives

#### Scenario: De-dupe against a tmux twin
- **WHEN** a session_id present in the native store also appears as a live tmux pane (e.g. after it was adopted)
- **THEN** only the tmux row SHALL be emitted; the native row SHALL be suppressed

#### Scenario: A record with no evidence behind it is withheld
- **WHEN** a native record names no live process (no sensed PID) and its session has no on-disk conversation
- **THEN** no row SHALL be emitted for it on any surface (agents/digest/app) — there is nothing a user could focus, kill, or adopt

#### Scenario: Idle time is tmux-independent
- **WHEN** a native session is idle
- **THEN** its "finished N ago" SHALL be computed from the session's own last logged message (the same session-keyed source used for tmux idle rows), not from tmux window activity

### Requirement: Native session lifecycle and reaping
The system SHALL remove a native-session record when the agent signals session end; SHALL remove a record the instant its recorded PROCESS is gone — the pid no longer exists, or is alive but running a DIFFERENT command than recorded (a pid-reuse guard) — independent of any grace; and SHALL otherwise treat a record as stale after a grace period past its last update. An idle-but-ALIVE native session SHALL persist (it is not reaped merely for being idle).

#### Scenario: Session end removes the record
- **WHEN** a `SessionEnd` (or equivalent end) hook fires for a native `session_id`
- **THEN** its native record SHALL be removed and it SHALL no longer appear in the radar

#### Scenario: Dead process is reaped immediately
- **WHEN** a native record's recorded process id no longer exists, or is alive but a different command (the pid was reused)
- **THEN** the record SHALL be removed at once, independent of the staleness grace — so a native agent that exited, was killed, or died in a reboot stops appearing immediately (not up to the grace later)

#### Scenario: Stale record is not shown
- **WHEN** a native record has not been updated within the staleness grace and no live signal exists
- **THEN** the radar SHALL omit it

### Requirement: Move a native session into tmux
The system SHALL provide a "Move to tmux" action that brings a native session under tmux by spawning a fresh tmux session — named after the agent's project (cwd basename) — that RESUMES the same conversation via the agent's resume command, reusing the existing resume/restore spawn path. It SHALL be offered ONLY for an **idle** native session that is resumable and whose `session_id` was captured AND whose conversation exists on disk; others SHALL be detect-only. After the resumed session is up, the system SHALL exit the ORIGINAL agent process (best-effort, guarded against pid reuse) so there is one live instance; it does not reparent the process or close the original terminal tab.

#### Scenario: Move an idle resumable native session
- **WHEN** the user moves an idle native session whose agent is resumable, whose `session_id` is known, and whose conversation is on disk
- **THEN** the system SHALL open a new tmux session (named after the project) running the agent's resume command, SHALL exit the original agent process, and the session SHALL thereafter be represented by the tmux row (its native row drops out)

#### Scenario: Move is unavailable for working / non-resumable / unpersisted sessions
- **WHEN** a native session is mid-turn (working), or its agent isn't resumable, or it has no on-disk conversation
- **THEN** the system SHALL NOT offer Move for it and SHALL still list it as sense-only

#### Scenario: The CLI accepts multiple sessions
- **WHEN** `gtmux adopt` is invoked with multiple session ids
- **THEN** it SHALL move each into its own tmux session

#### Scenario: The original process is exited, not the terminal
- **WHEN** a move completes
- **THEN** the system SHALL send the original agent process a terminate signal (only when it can still identify it), leaving the now-empty original terminal tab for the user to close

