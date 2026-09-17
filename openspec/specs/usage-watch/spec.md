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

Each window SHALL also carry its identity as DATA: a `kind` (`hour` | `session` | `day`
| `week` | `month` | `week-all` | `week-model`) and, for a per-model week, the `model`
as the agent spelled it; and a Claude window's printed reset SHALL be resolved to
`reset_unix` in the zone the line named, with the year taken from the clock (a reset is
at most a week ahead, so a time more than a day in the past belongs to next year). Both
are additive: a label of no known shape carries no kind, and a reset of no known shape
carries no epoch. The point is language: the label is the agent's own English, and a
surface reading it in Chinese SHALL word the window from `kind` (`本周（全部模型）`) and
the reset from `reset_unix` (`9月18日 22:59`) — the phone, the menu bar's reader and
`gtmux usage` alike — falling back to the printed words when the data is absent.

#### Scenario: A Chinese reader sees Chinese windows

- **WHEN** Claude prints `Current week (all models): 58% used · resets Sep 18 at 10:59pm (Asia/Shanghai)`
- **THEN** the window carries `kind:"week-all"` and a `reset_unix` for that instant in Asia/Shanghai
- **AND** the phone in Chinese shows `本周（全部模型）` and `9月18日 22:59`, in English the printed words

#### Scenario: Windows read from an agent's log

- **WHEN** a Codex rollout carries a rate-limit block with a 300-minute and a
  10080-minute window
- **THEN** they are reported as that agent's session and week windows
- **AND** a window whose reset has already passed is omitted

### Requirement: An agent whose plan cannot be read says so

A window that has ended is correctly omitted, but omission alone SHALL NOT be the
whole answer. An agent reporting through its own log goes quiet on its own the
moment it is not used: the last reading's windows roll over, every row for that
agent disappears, and nothing distinguishes that from the system having stopped
reading them. On 2026-09-07 an operator read exactly that as a defect.

The system SHALL therefore report, alongside the windows, any agent whose plan is
currently unreadable, naming the agent and the reason as a KEY that surfaces
translate. This SHALL be additive to the existing report so a shipped client that
does not know the field is unaffected.

It SHALL be reported only where the absence is news: the agent must have been used
recently enough that a missing figure means something, taking one week to match the
weekly window itself. An agent with no reading at all SHALL stay silent, since an
operator who does not use it must not be told about it.

A log-sourced agent SHALL be re-read on every path, the cache hit included. Its
source is a local file with nothing to amortise, and caching it both served windows
that had since ended and prevented a rolled-over reading from reporting itself.

#### Scenario: A log-sourced agent's last reading rolls over

- **WHEN** Codex's newest reading was written yesterday and both of its windows
  have since reset
- **THEN** no Codex window is reported as current
- **AND** the report names Codex as unreadable with reason "rolled-over"
- **AND** `gtmux limits` prints a line saying the window has ended and that one
  Codex turn brings the figure back

#### Scenario: An agent that has not been used in a month stays silent

- **WHEN** the newest Codex reading is thirty days old and expired
- **THEN** the report names no unreadable agent

#### Scenario: A fresh cache does not hide a log-sourced agent

- **WHEN** the Claude cache is within its TTL and carries a Codex window whose
  reset has since passed
- **THEN** that stale Codex window is not served
- **AND** the Codex reading is re-read from disk on that same call

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

Each window SHALL carry that judgement as data: `tier` is `warn` for a weekly window at or
past the warn threshold and `full` for any window at 100%, and is omitted otherwise. It is
computed when the report is read, from the threshold in force, so a cached snapshot is
judged by the current rule, and the warning line and the tier SHALL use one rule so they
can never name different windows.

#### Scenario: Weekly window near the cap warns

- **WHEN** a weekly window reports ≥ the warn threshold
- **THEN** `gtmux limits` marks it and one `limits·warn` wake reaches a live HQ,
  at most once per window per crossing

#### Scenario: Each window says where it stands

- **WHEN** the plan reads claude session 95%, claude week (all models) 66%, claude week
  (fable) 88% and codex week 100%
- **THEN** the session and the all-models week carry no tier, the fable week carries
  `warn`, and the codex week carries `full`

### Requirement: Tokens are totalled by local day across every agent

The system SHALL keep a daily token ledger: each usage message in every agent transcript
on the machine SHALL be attributed by its own timestamp to the local day it happened (a
cumulative log contributing the delta between consecutive totals), read incrementally
from a per-file byte watermark under a file lock, retaining a year (366 days). `gtmux usage
--json` and `GET /api/usage` SHALL carry `history`: the last seven local days oldest
first with per-agent counts, `today_out`/`today_in`, `week_out`/`week_in`, and the
week's split per agent with the registry's display name. `gtmux usage` SHALL print one
today/this-week line.

#### Scenario: Two days of two agents

- **WHEN** a Claude log carries 1,000 output tokens dated yesterday and 2,000 dated today,
  and a Codex log's cumulative totals go 500 → 800 today
- **THEN** `history.today_out` is 2,300, `history.week_out` is 3,300, and the week's split
  reads claude 3,000 · codex 300

#### Scenario: Reading twice counts once

- **WHEN** the ledger is updated, nothing is appended, and it is updated again
- **THEN** the totals are unchanged

### Requirement: The year at a glance

`history` SHALL also carry `activity` when the ledger knows any day: `since` (the first
day it knows, so "all" names its window rather than claiming a lifetime), `series` (every
day with output, oldest first, empty days omitted), `all_out`, `peak_out` with
`peak_date`, `streak` (consecutive days with output ending today, or yesterday while
today is still empty), `best_streak`, `active_days` and `days_known`. Every surface
SHALL draw the same picture from it, the way GitHub draws contributions: three figures
(today, this week, all since), a stats line (peak, streak, daily average, active days),
and a calendar heatmap of weeks across and Monday-to-Sunday down, levelled against the
window's peak in five steps of GitHub's green ramp (an empty day in the surface's own
faint ink), months labelled above, today ringed, with a weekly-bars and a cumulative view
of the same series beside it. The phone shows 20 weeks, the iPad and the menu bar's
reader 44, and `gtmux usage --activity` as many as the terminal is wide; `gtmux usage`
adds one all/peak/streak line. An older serve without `activity` leaves the seven-day
bars in place.

#### Scenario: A day is read off the calendar

- **WHEN** the ledger holds 4.1M on Aug 12 (the peak), 3.5M on Sep 8 and Sep 14, and 450k
  today, Tuesday Sep 15
- **THEN** Sep 15 sits in Tuesday's row of the last column, ringed, at level 1; Sep 8 at
  level 4; Wednesday of the last column is an empty slot; and tapping Sep 8 reads
  「9月8日 · 3.5M · claude 3.3M · codex 200k」
