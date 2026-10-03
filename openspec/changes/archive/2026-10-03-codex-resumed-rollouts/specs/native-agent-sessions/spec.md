## MODIFIED Requirements

### Requirement: Sense agent sessions running outside tmux

The system SHALL record the existence and state of an agent session that invokes `gtmux hook` while running outside tmux (no `$TMUX_PANE`), keyed by the agent's `session_id` rather than a tmux pane id. The record SHALL capture at least `{agent, sessionId, cwd, state, updatedAt}` — plus the agent process's pid and command name, used by the liveness reap below, and the hosting terminal app's display name ("Warp", "Ghostty", …) sensed best-effort from the hook's own environment/ancestry ("" when unrecognized) — where `state` is derived from the SAME hook lifecycle (`decide()`) used for tmux panes. The system SHALL NOT record an agent's internal warm-spare/pool process (e.g. Claude's `bg-spare`), which fires a hook but is never a real user-facing session. The system SHALL likewise NOT sense an agent's internal HELPER call (e.g. Codex's ambient-suggestions generator or auto-mode safety classifier), which fires a full pane-less hook lifecycle under a session id that is nobody's conversation: a pane-less `UserPromptSubmit` whose prompt head matches a known helper system prompt SHALL remove the session's native record, mark the session id, and swallow the session's later events — including from the event stream and the supervisor's unread debt (the already-streamed `SessionStart` is paired with a `SessionEnd` so the pane-less lifecycle-blink exclusion covers it). Detection SHALL rest on that positive prompt evidence, never on the empty pane alone (a real native session is pane-less too).

For a native Codex conversation, the system SHALL treat all rollout files whose
session metadata confirms the same conversation ID as one lifecycle. It SHALL
select the latest turn boundary by event timestamp, without accepting a
lookalike filename from another conversation.

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

#### Scenario: Codex completes without a usable native Stop hook
- **WHEN** a native Codex record says `working` but that same session's latest rollout turn boundary is `task_complete` newer than the working hook
- **THEN** the radar SHALL show the session as idle and allow its normal move action, without using a boundary from another session or overriding a newer `task_started`

#### Scenario: Codex completes in a resumed rollout
- **WHEN** a native Codex record is working and its conversation continues in a rollout named with the same session ID plus an instance suffix
- **THEN** the latest turn boundary across matching rollouts SHALL determine whether the session is working or idle, after verifying the suffixed rollout's own session metadata
- **AND** a later `task_complete` or `turn_aborted` SHALL make it idle, while a later `task_started` SHALL keep it working
- **AND** a lookalike file belonging to another session SHALL NOT affect its status

#### Scenario: Conversation activity spans rollouts
- **WHEN** a Codex conversation has more than one matching rollout
- **THEN** its first and last activity times and Chat turns SHALL cover the matching rollouts in chronological order
- **AND** the Chat cache validator SHALL change when any matching rollout grows
