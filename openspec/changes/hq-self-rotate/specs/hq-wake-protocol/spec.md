# hq-wake-protocol (delta)

## MODIFIED Requirements

### Requirement: Single wake channel with deterministic classes

All gtmux-initiated injections into the HQ pane SHALL flow through one wake
channel with exactly two classes: IMMEDIATE wakes for decision-dense events —
`waiting·<kind>`, `resolved` (a wait cleared), `asks`, `done` (unattended
completion), `crash` (a turn that died on an agent/API failure), `goal-changed`
(a user-direct prompt in a non-HQ pane), `new-session` (a newly sensed agent
session), `reap-suggest`, `feed-degraded`, and the standing resource/limits
warnings — and a periodic `tick` wake. The standing set SHALL additionally include the
periodic MAINTENANCE classes `distill` and `self-check`, the SESSION-HEALTH class
`self-rotate`, raised by the serve slow-tick's own sensors, and the completeness class
`unread`. No other event class SHALL be typed into
the HQ pane;
process-level events (prompt submissions, working transitions) reach HQ only by
pull (`gtmux digest`, `gtmux events`). Producer-heartbeat receipt suppression
(`feedSupersedesReceipts`) is REMOVED: the wake line is the only knock, and it
always fires for wake-class events (gated only on a live HQ pane and `hqNudge`).

The class list SHALL be understood as a PRIORITY vocabulary, not a coverage guarantee: a
class states what HQ should look at first, and NOT the set of events HQ can learn about.
Completeness is guaranteed separately, by the consumption watermark below. An event that
matches no class SHALL therefore still reach HQ.

#### Scenario: A process event does not touch the HQ screen

- **WHEN** an agent submits a prompt or transitions working→working in a non-HQ pane
- **THEN** no text is injected into the HQ pane; the event is only appended to the
  session-events log for later pull

#### Scenario: Wake-class events knock even when the feed daemon is healthy

- **WHEN** a wake-class event (e.g. `goal-changed`) occurs while the hq-feed daemon
  heartbeat is fresh
- **THEN** the wake line is still typed into the HQ pane (the daemon's health no
  longer suppresses the knock)

#### Scenario: An event matching no class still reaches HQ

- **WHEN** a session that gtmux never dispatched to reaches a turn-end that asks no
  question — matching neither `done` (no ledger entry) nor `asks`
- **THEN** no immediate class fires, and the event is nonetheless delivered to HQ by the
  unconsumed-events knock rather than being silently dropped

#### Scenario: The supervisor's own session is a wakeable subject

- **WHEN** the HQ session itself crosses a session-health threshold
- **THEN** a `self-rotate` wake is delivered on the same channel as every other class — the
  supervisor is not exempt from being the subject of a knock

## ADDED Requirements

### Requirement: A degraded HQ session wakes itself to rotate

gtmux SHALL observe the health of the SUPERVISOR'S OWN session — the one faculty no other
sensor watches — and SHALL wake HQ when that session is no longer fit to judge. The
observation SHALL be made by the resident serve process, never left to HQ's own initiative:
an event-driven supervisor is not running between wakes and has no timer, so a
self-assessment it must remember to perform is a self-assessment that never happens.

At least three facts SHALL be sensed, any one of which crossing its threshold is a breach:

- **context occupancy** — the session's live context fraction, the same figure `gtmux
  digest` reports as `ctx`;
- **session age** — measured from the session transcript's FIRST message, NOT from when
  gtmux first observed the session, so a serve restart cannot reset an old session's age to
  zero;
- **turn count** — HQ-pane prompt submissions since the current session began.

Thresholds SHALL be conservative by default (context ≥ 75%, age ≥ 12 h, turns ≥ 300) and
each SHALL be independently configurable under `hqWake` (`selfRotateCtx`,
`selfRotateHours`, `selfRotateTurns`), with a non-positive value disabling that criterion
alone.

On a standing breach gtmux SHALL deliver a `self-rotate` wake naming the breached figures,
at `PriorityStanding` — behind every decision-dense knock and evicted first at the queue
cap — repeating at `hqWake.selfRotateRepeatSec` (default 1800 s) for as long as the breach
stands. The wake SHALL be a complete no-op when no supervisor pane is resolvable.

The debt SHALL clear ONLY when the session actually rotates — observed as a CHANGE of the
HQ pane's agent session id. Delivering the wake SHALL NOT clear it, mirroring the
consumption watermark: gtmux stops asking only when the act it asked for has happened.

#### Scenario: A heavy session knocks

- **WHEN** the HQ session's context occupancy is at or past the configured fraction
- **THEN** a `» gtmux·self-rotate  ctx <n>% · <age> · <turns> turns │ …` line is delivered to
  the HQ pane

#### Scenario: A healthy session is silent

- **WHEN** every sensed figure is below its threshold
- **THEN** nothing is queued and nothing is written to the event stream

#### Scenario: No supervisor, no alarm

- **WHEN** the sensor runs with no HQ pane resolvable
- **THEN** nothing is queued, and no session-health verdict is reported as a failure — an
  absent supervisor is not a degraded one

#### Scenario: Knocking does not clear the debt

- **WHEN** a `self-rotate` wake has been delivered and the session id is unchanged
- **THEN** the breach still stands and the knock repeats at the repeat interval

#### Scenario: Rotation clears the debt

- **WHEN** the HQ pane's agent session id changes
- **THEN** the health window restarts from the new session — age, turns and the knock
  cadence all reset

#### Scenario: One criterion can be disabled without disabling the sensor

- **WHEN** `hqWake.selfRotateTurns` is set to 0 and the turn count is high while context and
  age are low
- **THEN** no wake is delivered, and a later context breach still knocks

### Requirement: HQ rotates itself through gtmux, not through raw tmux

`gtmux hq --rotate` SHALL resolve the live HQ pane itself and deliver that agent's own
conversation-reset input to it, and SHALL restart the session-health window so the knock
does not repeat about the session it just retired. It SHALL report a clear failure when no
HQ pane is resolvable.

This exists so the rotation stays inside HQ's role boundary: HQ decides and hands off, gtmux
performs the tmux mechanics, and the HARD role whitelist (HQ runs no concrete command, and
never sends navigation keys into a TUI) is not weakened to make self-rotation possible.

#### Scenario: Rotation is a gtmux verb

- **WHEN** HQ has brought its board and knowledge base current and run `gtmux hq --rotate`
- **THEN** gtmux delivers the reset input into the HQ pane and the health window restarts

#### Scenario: Rotation without a supervisor fails loudly

- **WHEN** `gtmux hq --rotate` runs with no live HQ pane
- **THEN** it exits non-zero with a message naming the missing supervisor, rather than
  silently succeeding
