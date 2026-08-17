# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: HQ subscribes to the silent feed and gates its own output

The seeded playbook SHALL teach HQ to perceive by PULL-ON-WAKE: on any wake line
it reads the delta (`gtmux events --since-seq <n>` and/or `gtmux digest --json`)
before acting. No background subscription exists — the journal is the single
stream, and a retention gap is announced by the pull itself (a CRITICAL warning
on the `--since-seq` read directing a digest-snapshot rebuild before acking).
HQ SHALL GATE its own user-visible output by surfacing tier: it SHALL print for
CRITICAL and NORMAL items (per the resolved threshold), and for QUIET items it
SHALL only record to the attention ledger and stay silent that turn. HQ SHALL
answer confirm-type asks itself only within the reversible ∧ low-risk ∧ no-fork
bound (recording the auto-answer), and escalate everything else. HQ SHALL always
surface a read-time gap warning regardless of the configured threshold.

#### Scenario: Wake then pull, on any agent

- **WHEN** HQ (running on any CLI agent, Claude or not) receives a wake line
  covering seq 341-352
- **THEN** it pulls the delta via CLI commands before acting — no background
  subscription is assumed

#### Scenario: A QUIET event produces no user output

- **WHEN** HQ ingests a QUIET-tier event from a pulled delta
- **THEN** it records the item in the ledger and prints nothing to the user that turn

#### Scenario: A CRITICAL event is surfaced

- **WHEN** HQ ingests a CRITICAL-tier event (a decision-type ask, a crash, or a
  read-time gap warning) — even while quiet mode is on
- **THEN** HQ prints it; a gap warning is surfaced despite the configured
  threshold, after reconciling from a digest snapshot

### Requirement: HQ verifies perception self-heal before nagging or restarting

The seeded playbook SHALL teach HQ that a `wake-degraded` wake reports that
gtmux's OWN mechanical self-heal has ALREADY run — it is a report, not a
request for HQ to restart anything. HQ SHALL first VERIFY by pulling the live
digest/events: when perception is actually fresh, HQ SHALL stay silent (record
only) and SHALL NOT repeatedly nag the user to restart. Only when the data is
genuinely stale/broken SHALL HQ act, and per the role boundary it SHALL restart
nothing itself — it SHALL escalate to the user with what it verified.

#### Scenario: Fresh perception after a degraded wake stays silent

- **WHEN** a `wake-degraded` wake arrives but a pull of `gtmux digest`/`events`
  shows perception is current (the mechanical self-heal recovered)
- **THEN** HQ records it and stays silent — it does not nag the user to restart

#### Scenario: A genuinely broken feed is restarted via a worker, not by HQ

- **WHEN** a `wake-degraded` wake arrives and the pulled data is genuinely stale
- **THEN** HQ escalates to the user with what it verified and restarts nothing
  itself — recovery is gtmux's mechanical job, not the supervisor's
