# hq-attention-system Specification

## Purpose
TBD - created by archiving change hq-attention-system. Update Purpose after archive.
## Requirements
### Requirement: Split feeding-HQ from showing-user

The system SHALL feed the supervisor the FULL event stream through a channel that is
NOT visible in the HQ pane, so that the only user-visible action is HQ's deliberate
print. gtmux SHALL NOT force-type low-value (`routine`/QUIET) event lines into the HQ
pane as a way to inform HQ. HQ's awareness of an event SHALL be independent of whether
the user is shown anything about it.

#### Scenario: Low-value events reach HQ silently

- **WHEN** a QUIET-tier event (e.g. a resolved wait, a send-landed confirmation, a
  working tick) occurs and an HQ pane is live
- **THEN** HQ receives it through the silent feed and gtmux does NOT type a visible
  `» gtmux·<class>` wake line for it into the HQ pane

#### Scenario: HQ omniscience is decoupled from user surfacing

- **WHEN** any event occurs while an HQ pane is live
- **THEN** the event is delivered to HQ regardless of surfacing tier, and only a
  CRITICAL/NORMAL judgment by HQ produces user-visible output

### Requirement: Perception feed daemon

The system SHALL provide a gtmux-managed, LLM-free perception daemon (`gtmux hq-feed`)
that tails the event journal from a persisted cursor and appends each event to a spool
file it owns under `~/.local/share/gtmux/hq-feed/`. The daemon SHALL be a singleton
guarded by a pidfile, SHALL keep the spool ROLLABLE (rotated at a size cap, never an
unbounded single file), and SHALL provide a cursor/rotation-aware tail mode HQ subscribes
to. Starting the daemon while one already runs SHALL NOT create a second.

#### Scenario: The daemon spools journal events

- **WHEN** `gtmux hq-feed` runs and a new event is appended to the journal
- **THEN** the daemon appends the event to its spool and advances its cursor

#### Scenario: The spool stays bounded

- **WHEN** the spool reaches its size cap
- **THEN** it is rotated to a retained generation and a fresh spool starts, so total
  on-disk size stays bounded

#### Scenario: Singleton

- **WHEN** `gtmux hq-feed --daemon` is invoked while a live daemon already holds the
  pidfile
- **THEN** the second invocation does not start a competing daemon

### Requirement: Zero-loss cursor catch-up and gap detection

On (re)start the feed SHALL resume from its consumed cursor and replay EXACTLY the events
whose sequence is greater than the cursor, so a crashed or restarted feed loses no events.
The feed SHALL detect a gap (a hole in consumed sequence numbers) and, on a gap, trigger a
reconciliation (a full digest snapshot) rather than silently continuing.

#### Scenario: Restart replays missed events

- **WHEN** the feed stops after consuming up to sequence N and later restarts while the
  journal has advanced to N+K
- **THEN** on restart it emits the events with sequence in (N, N+K] and none twice

#### Scenario: A gap triggers reconciliation

- **WHEN** the consumed sequence jumps (a missing sequence number is observed)
- **THEN** the feed marks a gap and requests a full reconciliation snapshot instead of
  proceeding as if nothing was missed

### Requirement: Heartbeat and mechanical watchdog

The feed daemon SHALL write a heartbeat every 30 s. A gtmux-side, LLM-free watchdog
(running in the `gtmux serve` slow-tick) SHALL, only while an HQ pane is live, treat a
missing pidfile or a heartbeat older than 90 s as a dead feed and mechanically restart the
daemon. A mechanical restart SHALL be SILENT (it does not disturb HQ). Only after two
consecutive restart attempts fail SHALL the watchdog escalate.

The mechanical restart SHALL NOT respawn the daemon on every tick during a persistent
outage. Restarts SHALL be spaced by an exponential backoff (widening from a base delay,
capped at a maximum), and after a bounded number of restart attempts within ONE continuous
outage the watchdog SHALL STOP attempting further restarts and rely on the CRITICAL
degradation plus the polling backstop instead of churning a daemon that will not come up.
The backoff and attempt count SHALL reset the moment the feed is healthy again (or no HQ
is live), so a later outage begins with an immediate restart.

#### Scenario: A stale feed is restarted silently

- **WHEN** the daemon's heartbeat is older than 90 s and an HQ pane is live
- **THEN** the watchdog restarts the daemon and does not print anything to the HQ pane

#### Scenario: A healthy feed is left alone

- **WHEN** the daemon's heartbeat is fresh (≤ 90 s)
- **THEN** the watchdog takes no action

#### Scenario: Repeated self-heal failure escalates

- **WHEN** two consecutive restart attempts fail to bring the heartbeat fresh
- **THEN** the watchdog raises a degradation (see the degradation requirement) rather
  than continuing to retry silently forever

#### Scenario: Restarts back off and stop after a cap

- **WHEN** the feed stays unhealthy across many ticks
- **THEN** the watchdog does not respawn every tick — attempts are spaced by a widening
  backoff and cease after the attempt cap, leaving the CRITICAL degradation and the
  polling backstop in effect

#### Scenario: Recovery resets the backoff

- **WHEN** the feed becomes healthy again after a backed-off / capped outage
- **THEN** the attempt count and backoff reset, so the next outage is restarted at once

### Requirement: Degradation is surfaced as CRITICAL

The system SHALL surface any perception-layer degradation to the user as a CRITICAL
condition immediately — the feed down after failed self-heal, a stale stream, or a
detected cursor gap — via a synthetic `feed-degraded` control record marked `important` in
the spool AND one visible HQ-pane nudge, so a perception outage is known within seconds
rather than discovered long after. Recovery SHALL clear the degradation state without
re-alerting on the recovery.

#### Scenario: An outage is announced at once

- **WHEN** the feed is judged down (failed self-heal) or the stream is stale
- **THEN** a CRITICAL degradation is surfaced to the user immediately, stating the feed
  is down and a polling backstop is in effect

#### Scenario: Recovery does not re-alert

- **WHEN** a previously degraded feed becomes healthy again
- **THEN** the degradation state clears and no new alert fires for the recovery itself

### Requirement: Startup reconciliation

On every (re)start of the perception feed, the system SHALL rebuild state from two sources
— replay the journal from the cursor AND pull one full `digest` snapshot — so a single
restart never loses state. The reconciliation SHALL be idempotent (safe to run on a
spurious gap).

#### Scenario: A restart rebuilds without loss

- **WHEN** the feed (re)starts
- **THEN** it replays outstanding journal events from the cursor and takes one full
  digest snapshot, reconstructing the current fleet state

### Requirement: Attention ledger

The system SHALL extend `gtmux tasks` into a general attention ledger. Each entry SHALL
additionally carry a surfacing `tier`, a re-orderable `priority`, a `surfaced` marker, and
a free-text `disposition`, plus first-seen / last-update timestamps — all additive and
optional so a legacy entry still loads. Priority SHALL be re-orderable and an entry SHALL
be able to be promoted after first being recorded (late promotion), so a QUIET item that
accrues related events can be surfaced later. Closed entries SHALL be archivable so the
live ledger stays small (rollable), and `gtmux tasks --verbose` SHALL retro-query the full
ledger including archived and disposition detail.

#### Scenario: An entry carries attention fields

- **WHEN** an attention item is recorded in the ledger
- **THEN** it stores its tier, priority, surfaced marker, disposition, and timestamps,
  and an older entry lacking these still loads

#### Scenario: Late promotion

- **WHEN** a QUIET-recorded item later accrues related events past a threshold
- **THEN** its priority/tier can be raised and it can be surfaced, without creating a
  duplicate entry

#### Scenario: Archived entries stay retro-queryable

- **WHEN** a ledger entry is closed and archived
- **THEN** the live `gtmux tasks` list no longer shows it but `gtmux tasks --verbose`
  can still retrieve it

### Requirement: Surfacing configuration

The system SHALL let the user tune the surfacing threshold. `config.json` SHALL support a
`surfaceTier` (`critical`|`normal`|`quiet`, default `normal` = surface NORMAL and above)
and a `quiet` toggle equivalent to raising the threshold to CRITICAL-only. The system
SHALL provide `gtmux quiet [on|off|status]` as the front door, and SHALL expose the
resolved threshold so HQ gates its prints accordingly. The threshold SHALL NEVER suppress
a degradation CRITICAL.

#### Scenario: Quiet mode raises the bar

- **WHEN** `gtmux quiet on` is set
- **THEN** the resolved surfacing threshold becomes CRITICAL-only and NORMAL items are
  ledger-recorded without a user-visible print

#### Scenario: Default threshold matches today's attention level

- **WHEN** no surfacing config is set
- **THEN** the resolved threshold is NORMAL-and-above

#### Scenario: A degradation is never quieted

- **WHEN** the surfacing threshold is CRITICAL-only (quiet on) and a feed degradation
  occurs
- **THEN** the degradation is still surfaced (it is CRITICAL and cannot be suppressed)

### Requirement: Self-check triggers sensed by gtmux

The system SHALL sense, LLM-free in the slow-tick, when HQ should run a self-check and
raise a `self-check` trigger to HQ (delivered as a feed control record, not counted as
user-facing). A trigger SHALL be raised when: the machine has been idle ≥ ~2 h with no
CRITICAL/NORMAL surfaced AND ≥ ~12 h since the last self-check (the resting-user case); OR
a threshold trips (open ledger entries over a cap, the journal over its rotation ceiling,
or a cursor gap); OR a daily floor (≥ 24 h since the last self-check). Triggers SHALL be
rate-limited to at most one per hour.

#### Scenario: Resting-user idle trigger

- **WHEN** the machine has been idle ≥ ~2 h with nothing surfaced and it has been ≥ ~12 h
  since the last self-check
- **THEN** gtmux raises a self-check trigger to HQ

#### Scenario: Threshold trigger fires immediately

- **WHEN** the open ledger exceeds its cap or the journal exceeds its rotation ceiling
- **THEN** gtmux raises a self-check trigger without waiting for idle

#### Scenario: Rate limited

- **WHEN** conditions would raise a second self-check trigger within an hour of the last
- **THEN** no second trigger is raised until the hour has elapsed

