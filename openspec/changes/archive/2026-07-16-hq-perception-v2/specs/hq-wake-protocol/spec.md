# hq-wake-protocol — delta

## ADDED Requirements

### Requirement: Single wake channel with deterministic classes

All gtmux-initiated injections into the HQ pane SHALL flow through one wake
channel with exactly two classes: IMMEDIATE wakes for decision-dense events —
`waiting·<kind>`, `resolved` (a wait cleared), `asks`, `done` (unattended
completion), `crash` (a turn that died on an agent/API failure), `goal-changed`
(a user-direct prompt in a non-HQ pane), `new-session` (a newly sensed agent
session), `reap-suggest`, `feed-degraded`, and the standing resource/limits
warnings — and a periodic `tick` wake. No other event class SHALL be typed into
the HQ pane;
process-level events (prompt submissions, working transitions) reach HQ only by
pull (`gtmux digest`, `gtmux events`). Producer-heartbeat receipt suppression
(`feedSupersedesReceipts`) is REMOVED: the wake line is the only knock, and it
always fires for wake-class events (gated only on a live HQ pane and `hqNudge`).

#### Scenario: A process event does not touch the HQ screen

- **WHEN** an agent submits a prompt or transitions working→working in a non-HQ pane
- **THEN** no text is injected into the HQ pane; the event is only appended to the
  session-events log for later pull

#### Scenario: Wake-class events knock even when the feed daemon is healthy

- **WHEN** a wake-class event (e.g. `goal-changed`) occurs while the hq-feed daemon
  heartbeat is fresh
- **THEN** the wake line is still typed into the HQ pane (the daemon's health no
  longer suppresses the knock)

### Requirement: Unattended completions wake HQ immediately with a judgment payload

A turn-end that leaves a session idle SHALL fire an immediate `done` wake when the
session's pane is NOT the focused pane of an attached tmux client at that moment
(the human was not watching it finish). The wake line SHALL carry enough to judge
without a drill: pane id and location, turn duration, the session's goal, and the
reply tail summary, each marked as DATA. A completion in the focused pane of an
attached client SHALL NOT fire an immediate wake and SHALL count toward the tick
tally instead. The behavior SHALL be configurable (`hqWake.done`:
`unattended` default | `always` | `tick`).

#### Scenario: A background session finishes

- **WHEN** a session in an unfocused window reaches idle after work
- **THEN** HQ receives one `done` wake line carrying loc, duration, goal, and the
  reply tail

#### Scenario: A completion under the user's eyes stays quiet

- **WHEN** a session finishes in the pane the user currently has focused in an
  attached client
- **THEN** no immediate wake fires and the completion is included in the next tick
  brief

### Requirement: Per-pane merge window bounds wake frequency

Immediate `done` wakes for the SAME pane SHALL be merged within a configurable
minimum-gap window (default 120s, `hqWake.paneMinGapSec`): a second completion
inside the window replaces/updates the queued line rather than adding another,
so a rapid-loop session cannot flood HQ's screen or token budget. Distinct panes
are not merged against each other (beyond the channel's existing coalescing).

#### Scenario: A rapid-fire session cannot flood HQ

- **WHEN** one pane produces five done transitions within the merge window
- **THEN** at most one wake line for that pane reaches HQ, carrying the newest
  payload

### Requirement: Summary tick with a zero-change gate

The system SHALL schedule a periodic summary tick (default every 10 minutes,
`hqWake.tickMinutes`), delivered as a `tick` wake line carrying the covered
sequence range and outcome counts — but ONLY when at least one outcome-level
change (done, new session, session gone, stall suspicion) accumulated since the
last delivered tick. With zero accumulated outcomes the tick SHALL be skipped
entirely: no wake, no injection, no tokens. An accumulation burst threshold
(default 5 outcomes, `hqWake.tickBurst`) SHALL fire the tick early.

#### Scenario: A quiet interval costs nothing

- **WHEN** the tick interval elapses with no outcome-level changes since the last
  delivered tick
- **THEN** nothing is injected into the HQ pane and no HQ turn occurs

#### Scenario: Accumulated outcomes trigger the early tick

- **WHEN** the fifth outcome-level change accumulates before the interval elapses
- **THEN** the tick wake is delivered early with the sequence range and counts

### Requirement: Signal visual language for injected lines

Every injected wake line SHALL open with the sigil `»` followed by
`gtmux·<class>` and columnar fields separated by `│`, so signal traffic is
visually distinct from conversational text in the HQ pane at a glance. The sigil
and separators SHALL be chosen for encoding robustness (survive a missing/POSIX
locale) and the format SHALL be pinned by a fixture test. Agent-authored content
in the line (titles, goals, reply tails) SHALL remain quoted-and-labelled as
DATA per the existing nudge-payload convention.

#### Scenario: A wake line is visually distinct

- **WHEN** any wake-class or tick line is injected into the HQ pane
- **THEN** it opens with `» gtmux·<class>` and uses `│`-separated fields

#### Scenario: The format survives a hostile locale

- **WHEN** the wake line is rendered under a POSIX/C locale environment
- **THEN** the sigil and separators still render without mojibake (pinned by test
  fixtures)

### Requirement: Consumer staleness never suppresses the wake

The wake channel SHALL track when HQ last pulled the stream (a pull stamp
recorded when delta-read commands run from the HQ home) and SHALL NOT suppress
any wake based on producer-side health (the old daemon-heartbeat suppression is
removed). When HQ's last pull is older than a staleness threshold, the wake line
SHALL additionally carry a pull-overdue hint naming the exact catch-up command;
wake lines themselves remain payload-rich (self-contained) in all cases.

#### Scenario: A stale consumer still gets woken, with a catch-up hint

- **WHEN** a wake-class event fires while HQ has not pulled the stream for longer
  than the staleness threshold
- **THEN** the wake line is still injected and carries a pull-overdue hint with
  the catch-up command
