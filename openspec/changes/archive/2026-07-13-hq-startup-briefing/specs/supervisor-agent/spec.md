# supervisor-agent Specification

## ADDED Requirements

### Requirement: HQ opens with a self-introduction and status briefing

When `gtmux hq` FRESH-spawns a supervisor session, the system SHALL deliver a
one-shot startup prompt into the new pane — after the agent comes up, via the
verified dispatch path (wait-for-ready, then a land-verified deliver, the same path
`gtmux spawn` uses) — so the supervisor's FIRST output does two things: (a) it
introduces itself and its role (overseeing every coding agent on the machine —
sense · decide · dispatch · supervise · report — and curating the knowledge base),
and (b) it produces an immediate status report grounded in `gtmux digest --json`,
`gtmux usage --json`, and `gtmux limits --json`, leading with who needs the user
(needs-you), then who is working on what and who finished, and ALWAYS including the
token-usage rollup and the subscription-window room (the same report shape the
seeded playbook's status policy requires). The briefing SHALL run ONLY on a fresh
spawn: a `gtmux hq` that focuses an already-live supervisor SHALL NOT re-deliver it.
It SHALL be best-effort and non-fatal — a delivery that does not land SHALL NOT fail
`gtmux hq`, since the session is already up and usable. The prompt SHALL be bilingual
(follows `GTMUX_LANG`) and SHALL be opt-out-able via `GTMUX_HQ_BRIEF`
(`off`/`0`/`false`/`no`), defaulting on.

#### Scenario: A fresh spawn briefs on the first turn

- **WHEN** `gtmux hq` spawns a new supervisor session and the agent comes up
- **THEN** a startup prompt is delivered into its pane so the supervisor's first
  output introduces itself and reports the fleet status (needs-you first, who's
  working, token usage + subscription room)

#### Scenario: A focused live supervisor is not re-briefed

- **WHEN** `gtmux hq` runs while a supervisor session is already live
- **THEN** it focuses the existing session and NO startup briefing is delivered

#### Scenario: Opt-out spawns HQ silently

- **WHEN** `GTMUX_HQ_BRIEF` is `off`/`0`/`false`/`no` and `gtmux hq` fresh-spawns
- **THEN** no startup briefing is delivered — the supervisor waits at its prompt

#### Scenario: A briefing that cannot land does not fail the command

- **WHEN** the agent does not come up in time, or the delivery cannot be verified
- **THEN** `gtmux hq` still succeeds (the session is up and usable) rather than
  reporting failure
