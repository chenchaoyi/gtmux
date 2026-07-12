# supervisor-agent Specification

## ADDED Requirements

### Requirement: HQ role boundary — sense/decide/dispatch/supervise/report only

The generated HQ playbook SHALL encode a hard role boundary: the supervisor does
NOT perform engineering work itself — it does not write code, run builds, or
change repositories. Its permitted verbs are sensing (read-only: `gtmux digest`,
`tmux capture-pane`), deciding, dispatching (`gtmux spawn` / `gtmux send`),
supervising, and reporting. Engineering work is delegated to the agents HQ
dispatches. Being a generated-once seed the user may edit, this is the DEFAULT
policy, not an enforced lock.

#### Scenario: The playbook forbids HQ doing the work itself

- **WHEN** the HQ home is seeded
- **THEN** the playbook states HQ does not write code / run builds / change repos,
  and instead dispatches that work to an agent

### Requirement: HQ dispatches through the verified path, never a raw launch

The HQ playbook SHALL direct the supervisor to dispatch new work through
`gtmux spawn` (which applies the proxy by construction and verifies delivery),
never a hand-rolled `send-keys` launch that would bypass the network-aware proxy
and 403. The `environment.md` knowledge seed SHALL state that the auto-proxy
covers ONLY gtmux's own launch path.

#### Scenario: Playbook points dispatch at `gtmux spawn`

- **WHEN** the HQ playbook and knowledge seeds are generated
- **THEN** they instruct dispatch via `gtmux spawn` and note that a bare
  `send-keys` launch is un-proxied and will 403

### Requirement: HQ never sends navigation keys into an agent TUI

The HQ playbook SHALL prohibit sending navigation keys (arrows, Tab, Page,
mode-switch keys) into an agent's TUI. A form or screen HQ cannot read SHALL be
surfaced to the user via `gtmux focus` rather than blind-driven — HQ does not
guess at multi-screen navigation it cannot see.

#### Scenario: Unreadable form is handed to the user

- **WHEN** an agent presents a multi-screen form or view HQ cannot read
- **THEN** the playbook has HQ `gtmux focus` it for the user, not send nav keys

### Requirement: Dispatch ledger nudges HQ on done/stuck

Every dispatch SHALL be tracked in the ledger (see agent-dispatch). When a tracked
task's pane finishes (idle-after-work) or stalls, the HQ nudge channel SHALL inform
the live supervisor, gated on a live HQ pane and the `hqNudge` setting, deduped so
a state is not re-nudged.

#### Scenario: A finished dispatch nudges HQ

- **WHEN** a tracked task's pane transitions to idle-after-work and an HQ pane is live
- **THEN** HQ receives a `done` nudge for that pane (once)

### Requirement: Waiting-resolved nudge and stale-chase retraction

The HQ nudge channel SHALL fire on BOTH edges of a wait. When a pane LEAVES
`waiting` for `working`/`idle` (e.g. the user answered directly in the pane's own
window, or the agent resumed), the system SHALL type a `resolved` nudge to a live
HQ carrying the pane and a short summary of the original ask, deduped so exactly
one resolve fires per wait (the waiting marker's existence at the transition edge
is the dedup). The corresponding dispatch/needs-you ledger entry SHALL be settled
(auto-cleared) on that transition. The HQ playbook SHALL instruct the supervisor
that, on a `resolved` nudge, it RETRACTS any pending relay or chase about that
pane — the matter was already handled.

#### Scenario: Answering in-pane clears the chase

- **WHEN** a waiting pane leaves `waiting` (its waiting marker existed and is
  cleared) and an HQ pane is live
- **THEN** HQ receives one `resolved` nudge for that pane, the ledger entry is
  settled, and the playbook has HQ drop any pending chase about it

#### Scenario: Resolve fires at most once

- **WHEN** the waiting→working/idle transition is processed and then a later event
  also clears waiting (a no-op, the marker is already gone)
- **THEN** no second `resolved` nudge is fired

### Requirement: HQ triages every turn-end response

The HQ playbook SHALL instruct the supervisor to sense EVERY agent turn-end
response — not only menu/permission waits — by subscribing to the session-events
stream (e.g. `gtmux events --follow`) and reacting to `asks` nudges. It SHALL triage
each response: a reply that asks a question → relay it to the user, obtain the
decision, and backfill the answer to the agent; a reply reporting completion →
acceptance-verify and report to the user; anything else → record without disturbing
the user. This closes the gap where a question embedded in reply text (raising no
menu) left HQ blind.

#### Scenario: A reply-text question is triaged to the user

- **WHEN** an agent's turn-end reply asks a question (no menu raised) and HQ is nudged
- **THEN** the playbook has HQ relay the question to the user, get the decision, and
  backfill the answer to the agent — not leave it unhandled

#### Scenario: A completion is acceptance-reported, progress is not noise

- **WHEN** a turn-end reply reports completion versus mere progress
- **THEN** the playbook has HQ acceptance-verify + report the former, and merely
  record the latter without disturbing the user

### Requirement: Reclaim is suggest → approve → execute

The HQ playbook SHALL instruct the supervisor that reclaiming a finished dispatch
(its session/worktree/branch) is always suggest → user approves → execute: on a
`reap-suggest`, HQ PROPOSES the reclamation to the user, naming the
session/worktree/branch and the exact `gtmux reap` command, and runs it ONLY after
the user approves. HQ SHALL NOT auto-delete sessions, worktrees, or branches. When the
user DECLINES a suggestion, the playbook SHALL have HQ snooze the candidate
(`gtmux reap --snooze`) and stop re-suggesting it until the snooze lapses — a user's
"keep it" is a decision to remember, not to re-litigate each tick.

#### Scenario: HQ proposes reclaim and waits

- **WHEN** HQ receives a `reap-suggest` for a finished dispatch
- **THEN** the playbook has HQ propose the `gtmux reap` command to the user and run
  it only after approval — never delete automatically

#### Scenario: A declined suggestion is snoozed, not re-nagged

- **WHEN** the user declines a `reap-suggest`
- **THEN** the playbook has HQ `gtmux reap --snooze` the candidate and not re-suggest
  it until the snooze lapses
