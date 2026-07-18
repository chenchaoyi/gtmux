# hq-attention-system — delta

## MODIFIED Requirements

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
