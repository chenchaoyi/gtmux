# usage-watch Specification

## Purpose
TBD - created by archiving change usage-watch. Update Purpose after archive.
## Requirements
### Requirement: Deterministic per-session usage extraction

The system SHALL compute, per agent session and without any LLM call, from the
session's own transcript: cumulative input/output tokens, the live CONTEXT
footprint (the last assistant message's input + cache_read + cache_creation
tokens) as a fraction of the model window, and a timestamp-based sliding-window
spend RATE (output tokens/min over the recent window). Sessions with no usage
data SHALL degrade to empty fields (Claude-first; other agents follow when their
logs carry usage).

#### Scenario: Session usage computed

- **WHEN** a Claude session has assistant messages with usage + timestamps
- **THEN** its usage row reports totals, context fraction, and the recent rate

### Requirement: Layered thresholds with ahead-of-time projection

The system SHALL evaluate each session against PER-AGENT-TYPE thresholds from
`~/.config/gtmux/usage.json` (sensible defaults when absent): context fraction,
per-session total burn, and per-agent-type aggregate rate. It SHALL also
PROJECT (current + rate × horizon) and flag a session/type whose projection
crosses a threshold BEFORE it is reached. The first breached-or-projected layer
is reported as a compact `usage_warn` string.

#### Scenario: Projected breach warns early

- **WHEN** a session's context is under the warn line but its rate projects
  crossing it within the horizon
- **THEN** its usage row carries a `usage_warn` naming the layer and the ETA

#### Scenario: Thresholds are per agent type

- **WHEN** usage.json sets different limits for claude vs codex
- **THEN** each session is judged against its own agent type's layers

### Requirement: Usage over CLI and API

The system SHALL provide `gtmux usage [--json]` (per-session rows + a
per-agent-type rollup) and the additive `GET /api/usage` (bearer-gated), byte-
consistent with the CLI JSON.

#### Scenario: Fleet usage at a glance

- **WHEN** `gtmux usage --json` (or the API) is called
- **THEN** every radar session appears with its usage fields and the rollup
  totals per agent type

### Requirement: Warnings reach the user and the supervisor

A breached or projected threshold SHALL surface as an amber usage MODIFIER on
the radar row (a modifier like errored/bg — never a status), and — when an hq
session is live — as one `[gtmux] usage·warn <loc> — <detail>` nudge line
(deduped per session+layer like the waiting nudge; `hqNudge:false` disables).

#### Scenario: Warn nudges the supervisor once

- **WHEN** a session first breaches (or projects into) a layer while HQ is live
- **THEN** one usage·warn line is typed into the HQ pane; an unchanged
  breach is not re-nudged

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

