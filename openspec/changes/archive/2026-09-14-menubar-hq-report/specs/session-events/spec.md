## ADDED Requirements

### Requirement: Acts-only event read

`gtmux events` SHALL accept `--acts`, which keeps only the records `events.IsSupervisorAct`
accepts — gtmux's own `gtmux:*` records minus the wake plumbing (`wake-delivered`,
`wake-dropped`) — the same partition `GET /api/hq/events?acts=1` applies, so a CLI reader
and an API reader count the same acts. It composes with `--since`, `--json`, `--follow`
and `--severity`. As a filtered read it SHALL NOT count as HQ's consumption.

#### Scenario: What the supervision did today

- **WHEN** `gtmux events --since 24h --acts --json` runs over a stream holding a fleet
  turn-end, a `gtmux:audit:send` and a `gtmux:audit:wake-delivered`
- **THEN** only the `gtmux:audit:send` record is printed
