# hq-wake-protocol (delta)

## ADDED Requirements

### Requirement: A standing knock re-validates its premise and backs off when nothing changed

Every wake class that repeats until an act clears it (`self-rotate`, `unread`,
`resource·warn`, `usage·warn`, `stuck·waiting`, and any future repeat-until-cleared
class) SHALL, before EACH delivery:

- **re-validate its premise** — re-sample the condition the line asserts, immediately
  before flushing it to the pane; when the premise no longer holds (the metric recovered,
  the value was externally reset, the wait resolved), the line SHALL NOT be delivered
  as-is (drop, or re-render against the fresh sample);
- **suppress an unchanged repeat** — when the breach set and rendered payload are
  identical to the last DELIVERED knock of that class AND the observed world has not
  changed since it (no new non-HQ fleet events, no HQ-pane turns), the repeat SHALL be
  suppressed; any change re-arms it. A periodic safety-floor re-check SHALL remain so a
  suppression bug cannot silence the class forever;
- **name its clearing act** — each class's definition SHALL state the act that clears the
  debt and which role can perform it, so a knock is never delivered to a consumer that
  cannot consume it.

Suppression SHALL never apply to an ESCALATION — a strictly more severe tier or a newly
breached criterion always delivers.

#### Scenario: A premise that evaporated in flight is not delivered

- **WHEN** a standing-class wake is queued and its premise probe, re-run at flush time,
  shows the asserted condition no longer holds (e.g. the value it warns about was reset
  externally)
- **THEN** the stale line is not delivered, and no debt is cleared by the drop

#### Scenario: An identical repeat against a static world is suppressed

- **WHEN** a standing-class knock would repeat with the same breach set and payload as the
  last delivered knock, and no fleet events or HQ turns occurred since
- **THEN** the repeat is suppressed, and the class re-arms on the first observed change

#### Scenario: An escalation is never suppressed

- **WHEN** a suppressed standing class crosses into a strictly more severe tier or breaches
  a new criterion
- **THEN** the knock is delivered immediately

## MODIFIED Requirements

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
cap. The wake SHALL be a complete no-op when no supervisor pane is resolvable.

Repeats SHALL follow the standing-knock rule (premise revalidation + unchanged-world
suppression) rather than a flat interval:

- **age SHALL NOT independently re-fire against a static, recovered session** — when
  context is back below its threshold and neither non-HQ fleet events nor HQ-pane turns
  occurred since the last delivered knock, an age-only (or age+turns-only, with context
  recovered) breach holds instead of knocking, because a healthy session with a current
  board must not be nagged indefinitely (measured: ~17 knocks across one 10-hour night
  with the fleet and handover unchanged from the 2nd knock on, the reply turns themselves
  pushing ctx 78%→96%; after an external compaction recovered ctx to 21% the knocking
  continued on age alone);
- an unchanged breach set SHALL suppress-until-change, re-armed by any fleet event, HQ
  turn, or a fresh criterion breach; the configured repeat cadence remains only as the
  safety-floor re-check.

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
- **THEN** the breach still stands, and the knock repeats only per the standing-knock rule
  (on change, or at the safety-floor re-check)

#### Scenario: Rotation clears the debt

- **WHEN** the HQ pane's agent session id changes
- **THEN** the health window restarts from the new session — age, turns and the knock
  cadence all reset

#### Scenario: One criterion can be disabled without disabling the sensor

- **WHEN** `hqWake.selfRotateTurns` is set to 0 and the turn count is high while context and
  age are low
- **THEN** no wake is delivered, and a later context breach still knocks

#### Scenario: Age alone does not nag a recovered, static session

- **WHEN** context has recovered below its threshold (e.g. an external compaction), the
  fleet has produced no non-HQ events and HQ no turns since the last delivered
  `self-rotate` knock, and only age (with or without turns) remains past its line
- **THEN** no repeat is delivered; the first fleet event, HQ turn, or fresh context breach
  re-arms the knock

#### Scenario: An unchanged breach does not re-knock on a timer alone

- **WHEN** a delivered `self-rotate` knock's breach set and figures are unchanged at the
  next repeat interval and the world is static
- **THEN** the repeat is suppressed until something changes, with only the safety-floor
  re-check still sensing
