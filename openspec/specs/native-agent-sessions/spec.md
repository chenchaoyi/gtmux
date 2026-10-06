# native-agent-sessions Specification

## Purpose
Sense agent conversations outside tmux, distinguish their client ownership, and safely move eligible sessions into tmux.

## Requirements

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

### Requirement: Native sessions appear in the radar as source "native"
`gtmux agents --json` SHALL include native sessions as rows with `source: "native"`, carrying agent, project (cwd), state, an idle "finished N ago" time, and the sensed hosting terminal name in the `terminal` field (omitted when unrecognized). These rows SHALL omit any focusable tmux locator and SHALL be marked as neither focusable nor send-able. A native session whose `session_id` also corresponds to a live tmux pane SHALL NOT be double-listed (the tmux row wins). A native row SHALL be listed only on positive evidence that something real is behind it — its record names a live process, or its session has an on-disk conversation; a record with neither (an unidentified helper call's residue) SHALL be withheld from every surface rather than shown as a convincing fake.

For Codex, the row MAY additionally carry `client: "chatgpt_desktop" | "terminal"` from the matching rollout's `session_meta.originator`: the exact names `codex_work_desktop` and `Codex Desktop` SHALL identify the desktop client, and `codex-tui` SHALL identify the terminal client. Missing, mismatched, or unrecognized metadata SHALL leave `client` absent. The rollout `source` field and cwd SHALL NOT be used to infer the client: `source: "vscode"` occurs for both clients. This client label is independent of `source: "native"` and does not change lifecycle or focus; desktop ownership prevents adoption as specified below.

#### Scenario: Native session listed alongside tmux ones
- **WHEN** a native session has a current record and no matching live tmux pane
- **THEN** `agents --json` SHALL include one row for it with `source: "native"` and no focusable locator

#### Scenario: Row names the hosting terminal
- **WHEN** a native session's hook fired from a recognized terminal app (e.g. a plain Warp window)
- **THEN** its radar row SHALL carry `terminal` with that app's display name (e.g. "Warp"), so the surfaces can label where the out-of-tmux agent lives

#### Scenario: Desktop Codex and terminal Codex stay distinct
- **WHEN** two native Codex sessions have matching rollouts with `originator` values `codex_work_desktop` and `codex-tui`, respectively
- **THEN** both remain `source: "native"`, while their `client` fields distinguish ChatGPT desktop from terminal Codex; an unknown originator yields no client label

#### Scenario: Both observed desktop originator names identify the same client
- **WHEN** native Codex sessions have matching rollouts with `originator` values `codex_work_desktop` and `Codex Desktop`, even when both have `source: "vscode"`
- **THEN** both rows SHALL carry `client: "chatgpt_desktop"` and SHALL NOT be adoptable; the direct CLI move SHALL refuse both without spawning or removing either native record

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
The system SHALL remove a native-session record when the agent signals session end; SHALL remove a record the instant its recorded PROCESS is gone — the pid no longer exists, or is alive but running a DIFFERENT command than recorded (a pid-reuse guard) — independent of any grace; and SHALL otherwise treat a record as stale after a grace period past its last update. An idle-but-ALIVE native session SHALL persist (it is not reaped merely for being idle). A process counts as alive only when it can be confirmed: its pid exists, its command can be read and matches the recorded command (when one was recorded), and its start time can be read and is not more than two minutes later than the record's last update. A pid now running a different command counts as gone, whether or not its start time can be read. A process that started more than two minutes after the record's last update did not write the record, whatever its command, and counts as gone. The two minutes absorb the start time's whole-second precision; this makes the start-time rule a presumption rather than proof of pid reuse, and a process with the recorded command name that took the pid within those two minutes cannot be told apart from the writer. A record whose process cannot be checked (no pid recorded, or its command or start time cannot be read) gets the grace period, and is never kept past it on missing evidence.

#### Scenario: Session end removes the record
- **WHEN** a `SessionEnd` (or equivalent end) hook fires for a native `session_id`
- **THEN** its native record SHALL be removed and it SHALL no longer appear in the radar

#### Scenario: Codex SessionEnd points at another session's pane
- **WHEN** a Codex `SessionEnd` names a native session but its inherited or same-directory candidate pane is not bound to that session
- **THEN** the native record SHALL still be removed, and the other pane SHALL keep its own state and SHALL NOT claim the end event

#### Scenario: Dead process is reaped immediately
- **WHEN** a native record's recorded process id no longer exists, or is alive but a different command (the pid was reused)
- **THEN** the record SHALL be removed at once, independent of the staleness grace — so a native agent that exited, was killed, or died in a reboot stops appearing immediately (not up to the grace later)

#### Scenario: Stale record is not shown
- **WHEN** a native record has not been updated within the staleness grace and no live signal exists
- **THEN** the radar SHALL omit it

#### Scenario: An idle session whose process is alive is kept

- **WHEN** a native session has sent no hook for longer than the grace period, and its
  recorded process is still running the recorded command and started before the record's
  last update
- **THEN** its record is kept and the session still shows

#### Scenario: A pid taken by a newer process is gone

- **WHEN** a record's pid now belongs to a process that started more than two minutes after
  the record's last update
- **THEN** the record is removed at once, even when that process runs a command of the same
  name

#### Scenario: A process that cannot be fully read gets only the grace

- **WHEN** a record's pid exists but its command or its start time cannot be read, and
  nothing read contradicts the record
- **THEN** the record is kept within the grace period and removed after it

### Requirement: Move a native session into tmux
The system SHALL provide a "Move to tmux" action that brings a native session under tmux by spawning a fresh tmux session — named after the agent's project (cwd basename) — that RESUMES the same conversation via the agent's resume command, reusing the existing resume/restore spawn path. It SHALL be offered ONLY for an **idle** native session that is resumable and whose `session_id` was captured AND whose conversation exists on disk; others SHALL be detect-only. ChatGPT desktop Codex sessions SHALL NOT offer this action, and a direct CLI attempt SHALL refuse before spawning: the desktop app owns the thread and its native hook record cannot identify an agent process to exit, so resuming into tmux would create a second client while claiming a move. After an eligible resumed session is up, the system SHALL exit the ORIGINAL agent process (best-effort, guarded against pid reuse) so there is one live instance; it does not reparent the process or close the original terminal tab.

#### Scenario: Move an idle resumable native session
- **WHEN** the user moves an idle native session whose agent is resumable, whose `session_id` is known, and whose conversation is on disk
- **THEN** the system SHALL open a new tmux session (named after the project) running the agent's resume command, SHALL exit the original agent process, and the session SHALL thereafter be represented by the tmux row (its native row drops out)

#### Scenario: Move is unavailable for working / non-resumable / unpersisted sessions
- **WHEN** a native session is mid-turn (working), or its agent isn't resumable, or it has no on-disk conversation
- **THEN** the system SHALL NOT offer Move for it and SHALL still list it as sense-only

#### Scenario: The command asks the same question as the radar
- **WHEN** `gtmux adopt <id>` names a session that is mid-turn (working or waiting), not resumable, or has nothing on disk — including one that was idle when the radar offered Move and has started a turn since
- **THEN** it SHALL refuse before creating any tmux session, with the same verdict the radar uses for Move (the Codex rollout's newer completion counting as idle there too), and SHALL leave the native record and the original process as they were

#### Scenario: A move that does not come up leaves the original
- **WHEN** after the new tmux session is created its pane cannot be found, the resume command cannot be typed into it, or the resumed agent does not take the pane over within the dispatch ready timeout
- **THEN** the system SHALL remove the tmux session it created, SHALL NOT exit the original process or drop its native record, and SHALL report the move as failed, so it can be tried again; if that session cannot be removed, the report SHALL name it and how to remove it, and SHALL NOT claim it was removed

#### Scenario: Desktop Codex stays in its owning app
- **WHEN** a native Codex row has `client: "chatgpt_desktop"`
- **THEN** Move SHALL be hidden; `gtmux adopt <id>` SHALL refuse without spawning or removing its native record

#### Scenario: The CLI accepts multiple sessions
- **WHEN** `gtmux adopt` is invoked with multiple session ids
- **THEN** it SHALL move each into its own tmux session

#### Scenario: The original process is exited, not the terminal
- **WHEN** a move completes
- **THEN** the system SHALL send the original agent process a terminate signal (only when it can still identify it, and only after the resumed agent took over its new pane), leaving the now-empty original terminal tab for the user to close

### Requirement: A pane-less hook is proven native before it is treated as native

The system SHALL check a pane-less hook's process ancestry for a tmux pane before recording a native session.

`$TMUX_PANE` is the hook's first signal for which pane it belongs to, but it is no longer
the only one. An agent may run its conversation in a process that does not inherit the
terminal's environment — Claude Code's background session host (`claude daemon` →
`--bg-pty-host`) is one such process, and it has neither a tty nor `$TMUX_PANE` while its
ancestry still leads back to the tmux pane the user is sitting in.

When `$TMUX_PANE` is absent, the system SHALL attempt to identify the pane from the hook
process's ancestry before recording a native session: it SHALL walk a bounded number of
ancestors and match them against tmux's own pane table, by `pane_pid` first and then by
`pane_tty`. A single unambiguous match SHALL be treated as that pane, exactly as if
`$TMUX_PANE` had named it. Ambiguity or no match SHALL fall through to the existing
native-session path, which stays the default for genuinely non-tmux agents.

The lookup SHALL be bounded and SHALL fail open: it runs only when `$TMUX_PANE` is
absent, walks at most a fixed number of ancestors, and any error resolves to "not
identified" rather than blocking the hook.

#### Scenario: A background-hosted session is still its pane's session

- **WHEN** a hook fires from a process with no `$TMUX_PANE` whose ancestry includes the
  process tmux reports as a pane's `pane_pid`
- **THEN** the event is recorded against that pane — its state, its resume binding, its
  event stream — and no native session record is written

#### Scenario: Only the tty survives the chain

- **WHEN** no ancestor pid matches a `pane_pid`, but exactly one pane's `pane_tty` matches
  an ancestor's tty
- **THEN** that pane is used

#### Scenario: A genuinely native agent is unaffected

- **WHEN** a hook fires with no `$TMUX_PANE` and no ancestor matches any pane
- **THEN** the session is recorded as a `source: "native"` row, as before

#### Scenario: Ambiguity is not a guess

- **WHEN** the ancestry matches more than one pane
- **THEN** no pane is claimed and the native path is taken
