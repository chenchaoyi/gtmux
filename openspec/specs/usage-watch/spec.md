# usage-watch Specification

## Purpose
TBD - created by archiving change usage-watch. Update Purpose after archive.
## Requirements
### Requirement: Deterministic per-session usage extraction

The system SHALL compute, per agent session and without any LLM call, from the
session's own transcript: cumulative input/output tokens, the live CONTEXT
footprint as a fraction of the model window, and a timestamp-based sliding-window
spend RATE (output tokens/min over the recent window). Sessions whose log carries
no usage SHALL degrade to empty fields.

The system SHALL support both shapes agent logs use, because reading one with the
other's arithmetic is silently wrong rather than empty:

- **per-message deltas** (Claude: each assistant message's own usage) — totals are
  the SUM, accumulated incrementally by byte offset so no caller rescans the log;
- **running totals** (Codex: a `token_count` event after each turn carrying the
  session's totals so far) — totals are the LAST reading. Summing them would
  multiply a session's burn by its turn count.

The context window SHALL be taken from the best available source, in order: a
configured per-agent override, a window the log STATES outright (Codex reports
`model_context_window`), then inference from evidence (the smallest known tier ≥
the observed footprint). A snapshot SHALL carry the window it was judged against,
so a later evaluation cannot re-derive a different one.

#### Scenario: Session usage computed from per-message deltas

- **WHEN** a Claude session has assistant messages with usage + timestamps
- **THEN** its usage row reports totals, context fraction, and the recent rate

#### Scenario: Session usage computed from running totals

- **WHEN** a Codex session's log carries `token_count` events
- **THEN** its totals are the last event's, not the sum of every event's
- **AND** its context fraction is measured against the window the log stated

### Requirement: Layered thresholds with ahead-of-time projection

The system SHALL evaluate each session against PER-AGENT-TYPE thresholds from
`~/.config/gtmux/usage.json` (sensible defaults when absent): context fraction,
per-session burn, and per-agent-type aggregate rate. It SHALL also
PROJECT (current + rate × horizon) and flag a session/type whose projection
crosses a threshold BEFORE it is reached. The first breached-or-projected layer
is reported as a compact `usage_warn` string.

A CUMULATIVE quantity SHALL NOT alarm on its total crossing a line. Session output only
ever grows, so such an alarm has no de-assert condition: once true it stays true for the
session's life, and a warning that cannot clear is a permanent label rather than a
warning. Burn SHALL therefore be judged on RATE — the projection, which becomes false
when the rate drops. A session already past the line SHALL still warn while it is
producing, naming the rate, and SHALL fall silent when it stops.

A repeated usage warning for the same pane SHALL be subject to a minimum restate
interval, independent of which LAYER is reported. Layer identity alone is not a dedup: the
first breached layer is reported, so a value dithering around one threshold changes which
layer speaks and every change reads as news. A momentary drop below a threshold SHALL NOT
be treated as a recovery that re-arms the alarm, for the same reason.

#### Scenario: Projected breach warns early

- **WHEN** a session's context is under the warn line but its rate projects
  crossing it within the horizon
- **THEN** its usage row carries a `usage_warn` naming the layer and the ETA

#### Scenario: A stopped session past the burn line is silent

- **WHEN** a session's cumulative output is past its burn threshold but the session is no
  longer producing
- **THEN** no burn warning is reported — the total alone is not an alarm

#### Scenario: A session past the line and still burning warns with its rate

- **WHEN** a session is past its burn threshold and still producing
- **THEN** the warning names the rate, so the condition can be seen to improve

#### Scenario: A dithering value does not re-announce itself

- **WHEN** a session's context crosses back and forth over its threshold within the
  restate interval, changing which layer is reported
- **THEN** at most one warning is delivered for that pane in the interval

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
session is live — as one `usage·warn` WAKE (deduped per session+layer like the
waiting wake; `hqNudge:false` disables).

The wake SHALL ride the single wake channel like every other injection: the declared
`usage·warn` class, the `» gtmux·<class>` signal format, and the channel's draft guard,
ack, and queue. It SHALL NOT be hand-built and typed into the pane directly — that path
had no draft guard, so a warning firing while the user was mid-sentence in HQ appended
itself to their draft AND submitted it.

#### Scenario: Warn nudges the supervisor once

- **WHEN** a session first breaches (or projects into) a layer while HQ is live
- **THEN** one `usage·warn` wake reaches the HQ pane; an unchanged breach is not
  re-nudged

#### Scenario: The warning cannot clobber a half-typed HQ draft

- **WHEN** a usage warning fires while the user is composing in the HQ pane
- **THEN** nothing is typed: the wake queues and lands once the box is empty, like every
  other wake

### Requirement: Subscription-window limits from whatever the agent itself reports

The system SHALL obtain real subscription-window usage as `{label, pctUsed,
resetAt}` per window, from the agent's OWN reporting — authoritative server data,
NOT local estimation and NOT a private endpoint. Absent or unparseable data SHALL
yield no limits for that agent, leaving the rest of usage working.

Agents report it in different places, and the system SHALL use whichever the
agent actually offers rather than assuming one mechanism:

- **A sanctioned command** (Claude): a configurable, cached command
  (default `claude -p "/usage"`), because Claude records nothing about windows
  locally — its transcript holds session COST, and its stats cache holds all-time
  model totals; neither knows a window or a reset.
- **The agent's own log** (Codex): the server's rate-limit response, recorded into
  the session rollout beside the token counts. Reading it costs no process and no
  command. Codex's own `/usage` reports activity history rather than remaining
  quota, so the command mechanism has no counterpart there.

A window SHALL be identified by its DURATION, never by its position in the source
(Codex's `primary` field is observed carrying both the 5-hour and the weekly
window). A window whose reset time has already passed SHALL NOT be reported as
current: a log-derived reading can outlive its own window, and an expired
percentage is unknown rather than low.

Each window SHALL carry which agent's plan it belongs to, in its LABEL as well as
a field — including the first agent's, since an unqualified label beside a
qualified one reads as the general case beside a special one. A consumer acting
on it (the dispatch preflight suggests a cheaper model) thereby acts on the plan
the work will actually bill against.

#### Scenario: Windows read from an agent's log

- **WHEN** a Codex rollout carries a rate-limit block with a 300-minute and a
  10080-minute window
- **THEN** they are reported as that agent's session and week windows
- **AND** a window whose reset has already passed is omitted

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

A FAILED run SHALL NOT be cached as fresh, and SHALL NOT be retried on every call
either. The system SHALL record the last attempt separately from the last success and
back off between failures (growing, capped at the TTL), so that a command which keeps
failing costs no more than one that is working. It SHALL bound each run with a timeout,
because the command spawns a real agent session and an unbounded one can be waited on
while the next call starts another beside it. An explicit user refresh SHALL bypass the
backoff.

#### Scenario: Fresh cache is reused

- **WHEN** `gtmux usage`/`gtmux limits` is called within the TTL of the last run
- **THEN** the cached windows are served without spawning the command again

#### Scenario: A failing command is not retried on every call

- **WHEN** the limits command fails and several callers ask for limits in quick
  succession
- **THEN** the command runs once, the last good windows and their success time are left
  untouched, and further runs wait out a growing backoff

#### Scenario: Recovery clears the backoff

- **WHEN** the command succeeds after a run of failures
- **THEN** the cache is written with a fresh success time and the next failure starts
  the backoff over from its shortest step

#### Scenario: A hung command is abandoned

- **WHEN** the limits command does not return within its timeout
- **THEN** it is killed, the call returns the last good snapshot, and the failure enters
  the backoff like any other

### Requirement: Limits surface and warn

The system SHALL surface the windows in `gtmux usage`/`gtmux limits` (+ `--json`
and `GET /api/usage`), and SHALL raise a warning (the amber usage modifier +, when
HQ is live, one `limits·warn` wake through the wake channel, deduped per window) when a
window crosses its configured threshold (default: any weekly window ≥ 85%).

#### Scenario: Weekly window near the cap warns

- **WHEN** a weekly window reports ≥ the warn threshold
- **THEN** `gtmux limits` marks it and one `limits·warn` wake reaches a live HQ,
  at most once per window per crossing
