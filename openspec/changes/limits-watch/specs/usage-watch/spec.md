## ADDED Requirements

### Requirement: Subscription-window limits from the agent's own usage command

The system SHALL obtain real subscription-window usage (e.g. Claude's 5-hour
session window and weekly windows) by running a configurable, cached command
(default `claude -p "/usage"`) and parsing each reported window into
`{label, pctUsed, resetAt}`. This is authoritative server data surfaced via the
agent's own sanctioned command — NOT local estimation and NOT a private endpoint.
Absent/unparuseable output SHALL yield no limits (the rest of usage still works).

#### Scenario: Windows parsed from /usage

- **WHEN** the limits command reports "Current week (all models): 58% used ·
  resets Jul 17 …"
- **THEN** the system records a window {label:"week (all models)", pctUsed:58,
  resetAt:"Jul 17 …"}

### Requirement: Limits are cached, not run per call

The system SHALL cache the parsed limits with a TTL (default 15 minutes, shortened to 5 minutes when any window is near its cap) because
obtaining them spawns a process; it SHALL refresh at most once per TTL on demand
(a `--refresh` flag forces one), and it SHALL NEVER spawn the command once per
`gtmux usage` invocation.

#### Scenario: Fresh cache is reused

- **WHEN** `gtmux usage`/`gtmux limits` is called within the TTL of the last run
- **THEN** the cached windows are served without spawning the command again

### Requirement: Limits surface and warn

The system SHALL surface the windows in `gtmux usage`/`gtmux limits` (+ `--json`
and `GET /api/usage`), and SHALL raise a warning (the amber usage modifier +, when
HQ is live, one `[gtmux] limits·warn …` nudge, deduped per window) when a window
crosses its configured threshold (default: any weekly window ≥ 85%).

#### Scenario: Weekly window near the cap warns

- **WHEN** a weekly window reports ≥ the warn threshold
- **THEN** `gtmux limits` marks it and one `limits·warn` line reaches a live HQ,
  at most once per window per crossing
