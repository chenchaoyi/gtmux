# HQ Wake Protocol Specification

## Purpose

The deterministic wake channel into the HQ (supervisor) pane: decision-dense
events knock with one visually-distinct signal line; everything else stays
pull-side (events/digest). It replaces per-event receipt forwarding and the
producer-heartbeat suppression with a single, bounded, zero-token-when-quiet
arousal mechanism — so HQ reacts in seconds to what matters, and its screen
stays silent otherwise.
## Requirements
### Requirement: Single wake channel with deterministic classes

All gtmux-initiated injections into the HQ pane SHALL flow through one wake
channel with exactly two classes: IMMEDIATE wakes for decision-dense events —
`waiting·<kind>`, `resolved` (a wait cleared), `asks`, `done` (unattended
completion), `crash` (a turn that died on an agent/API failure), `goal-changed`
(a user-direct prompt in a non-HQ pane), `new-session` (a newly sensed agent
session), `reap-suggest`, `feed-degraded`, and the standing resource/limits
warnings — and a periodic `tick` wake. No other event class SHALL be typed into
the HQ pane;
process-level events (prompt submissions, working transitions) reach HQ only by
pull (`gtmux digest`, `gtmux events`). Producer-heartbeat receipt suppression
(`feedSupersedesReceipts`) is REMOVED: the wake line is the only knock, and it
always fires for wake-class events (gated only on a live HQ pane and `hqNudge`).

#### Scenario: A process event does not touch the HQ screen

- **WHEN** an agent submits a prompt or transitions working→working in a non-HQ pane
- **THEN** no text is injected into the HQ pane; the event is only appended to the
  session-events log for later pull

#### Scenario: Wake-class events knock even when the feed daemon is healthy

- **WHEN** a wake-class event (e.g. `goal-changed`) occurs while the hq-feed daemon
  heartbeat is fresh
- **THEN** the wake line is still typed into the HQ pane (the daemon's health no
  longer suppresses the knock)

### Requirement: Unattended completions wake HQ immediately with a judgment payload

A turn-end that leaves a session idle SHALL fire an immediate `done` wake when the
session's pane is NOT the focused pane of an attached tmux client at that moment
(the human was not watching it finish), OR when the pane is AWAITED — a pane HQ
dispatched work to (via `gtmux spawn` or `gtmux send`) and is expecting a completion
from. An AWAITED completion SHALL fire the immediate wake EVEN when the pane is the
focused pane of an attached client, overriding the attended-defer, because HQ is
explicitly waiting on that pane and must not be left to a deferred tick. The wake line
SHALL carry enough to judge without a drill: pane id and location, turn duration, the
session's goal, and the reply tail summary, each marked as DATA. A completion in the
focused pane of an attached client that is NOT awaited SHALL NOT fire an immediate wake
and SHALL count toward the tick tally instead. The awaited flag SHALL be cleared once
the wake fires (a one-shot per dispatch). The behavior SHALL remain configurable
(`hqWake.done`: `unattended` default | `always` | `tick`); the awaited override applies
under the `unattended` default (the case the bug occurs in).

#### Scenario: A background session finishes

- **WHEN** a session in an unfocused window reaches idle after work
- **THEN** HQ receives one `done` wake line carrying loc, duration, goal, and the
  reply tail

#### Scenario: A completion under the user's eyes stays quiet

- **WHEN** a session finishes in the pane the user currently has focused in an
  attached client, and the pane is NOT awaited
- **THEN** no immediate wake fires and the completion is included in the next tick
  brief

#### Scenario: An HQ-awaited completion wakes immediately even when attended

- **WHEN** HQ drove a pane via `gtmux send` (or `spawn`) — so the pane is awaited — and
  that pane finishes while it is the focused pane of an attached client
- **THEN** HQ receives an immediate `done` wake on the acked wake channel (not a
  deferred tick tally), and the awaited flag is cleared

### Requirement: Per-pane merge window bounds wake frequency

Immediate `done` wakes for the SAME pane SHALL be merged within a configurable
minimum-gap window (default 120s, `hqWake.paneMinGapSec`): a second completion
inside the window replaces/updates the queued line rather than adding another,
so a rapid-loop session cannot flood HQ's screen or token budget. Distinct panes
are not merged against each other (beyond the channel's existing coalescing).

#### Scenario: A rapid-fire session cannot flood HQ

- **WHEN** one pane produces five done transitions within the merge window
- **THEN** at most one wake line for that pane reaches HQ, carrying the newest
  payload

### Requirement: Summary tick with a zero-change gate

The system SHALL schedule a periodic summary tick (default every 10 minutes,
`hqWake.tickMinutes`), delivered as a `tick` wake line carrying the covered
sequence range and outcome counts — but ONLY when at least one outcome-level
change (done, new session, session gone, stall suspicion) accumulated since the
last delivered tick. With zero accumulated outcomes the tick SHALL be skipped
entirely: no wake, no injection, no tokens. An accumulation burst threshold
(default 5 outcomes, `hqWake.tickBurst`) SHALL fire the tick early.

#### Scenario: A quiet interval costs nothing

- **WHEN** the tick interval elapses with no outcome-level changes since the last
  delivered tick
- **THEN** nothing is injected into the HQ pane and no HQ turn occurs

#### Scenario: Accumulated outcomes trigger the early tick

- **WHEN** the fifth outcome-level change accumulates before the interval elapses
- **THEN** the tick wake is delivered early with the sequence range and counts

### Requirement: Signal visual language for injected lines

Every injected wake line SHALL open with the sigil `»` followed by
`gtmux·<class>` and columnar fields separated by `│`, so signal traffic is
visually distinct from conversational text in the HQ pane at a glance. The sigil
and separators SHALL be chosen for encoding robustness (survive a missing/POSIX
locale) and the format SHALL be pinned by a fixture test. Agent-authored content
in the line (titles, goals, reply tails) SHALL remain quoted-and-labelled as
DATA per the existing nudge-payload convention.

#### Scenario: A wake line is visually distinct

- **WHEN** any wake-class or tick line is injected into the HQ pane
- **THEN** it opens with `» gtmux·<class>` and uses `│`-separated fields

#### Scenario: The format survives a hostile locale

- **WHEN** the wake line is rendered under a POSIX/C locale environment
- **THEN** the sigil and separators still render without mojibake (pinned by test
  fixtures)

### Requirement: Consumer staleness never suppresses the wake

The wake channel SHALL track when HQ last pulled the stream (a pull stamp
recorded when delta-read commands run from the HQ home) and SHALL NOT suppress
any wake based on producer-side health (the old daemon-heartbeat suppression is
removed). When HQ's last pull is older than a staleness threshold, the wake line
SHALL additionally carry a pull-overdue hint naming the exact catch-up command;
wake lines themselves remain payload-rich (self-contained) in all cases.

#### Scenario: A stale consumer still gets woken, with a catch-up hint

- **WHEN** a wake-class event fires while HQ has not pulled the stream for longer
  than the staleness threshold
- **THEN** the wake line is still injected and carries a pull-overdue hint with
  the catch-up command

### Requirement: Wake delivery is acknowledged, retried, and never silently dropped

A wake SHALL be removed from the delivery queue ONLY after its delivery is confirmed.
Delivery SHALL paste the line and submit it as separate steps (a paste buffer, then a
named Enter key), and SHALL confirm the batch reached the pane by reading the pane's
capture (including scrollback margin) for the batch's identifier. Any error from the
paste or the submit, and any unconfirmed read, SHALL return the batch to the queue for a
later attempt. A queue entry claimed by a drainer that never completed (a claim older
than 60 seconds) SHALL be reclaimed by the next drain.

#### Scenario: A failed send keeps the nudge

- **WHEN** the paste or the Enter of a wake batch returns an error
- **THEN** the batch is returned to the queue and delivered by a later drain, and no
  queue entry is deleted

#### Scenario: An unconfirmed delivery is retried

- **WHEN** a wake batch is pasted and submitted but its identifier does not appear in the
  pane capture
- **THEN** the batch is returned to the queue and re-attempted on the next drain

#### Scenario: An entry that can never be confirmed does not loop forever

- **WHEN** a batch is pasted and submitted successfully but its delivery is never
  confirmed, drain after drain
- **THEN** it is re-sent at most 3 times in total (each carrying the same identifier)
  and then dropped, with the degradation raised — a send that ERRORS instead (nothing
  reached the pane) keeps retrying without limit, since it risks no duplicate

#### Scenario: A crashed drainer's batch is reclaimed

- **WHEN** a drainer claims a queue entry and dies before delivering it
- **THEN** a later drain reclaims the claim once it is older than 60 seconds and delivers
  the entry

### Requirement: Each coalesced wake batch carries a re-send identifier

Every delivered wake line SHALL end with a short identifier derived from the batch's
queue entries and payload, so that a re-send of an unconfirmed batch carries the SAME
identifier while a genuinely new batch carries a different one. The identifier SHALL be
what the delivery confirmation matches on, and the supervisor playbook SHALL treat a
repeated identifier as a re-send to ignore. Delivery is at-least-once: a duplicate is
recognizable, a loss is not acceptable.

#### Scenario: A retried batch is recognizable as a duplicate

- **WHEN** an unconfirmed batch is re-delivered and both attempts reach HQ
- **THEN** both lines carry the same trailing identifier, which HQ uses to ignore the
  second

#### Scenario: A new batch with identical text is not a duplicate

- **WHEN** two separate wake events produce identical line text at different times
- **THEN** their batches carry different identifiers

### Requirement: A broken wake channel escalates out of band

Consecutive unconfirmed deliveries SHALL be counted, and reaching the failure limit
(3) SHALL raise a CRITICAL `wake-degraded` degradation exactly once per transition into
the degraded state: a control record at important severity, a best-effort HQ wake line,
and a desktop notification — the last because the alarm for a broken wake channel MUST
NOT depend on that channel. A confirmed delivery SHALL reset the counter, and recovery
SHALL NOT re-alert.

#### Scenario: The wake channel breaks

- **WHEN** three consecutive wake deliveries fail to confirm
- **THEN** a `wake-degraded` control record at important severity is emitted and a
  desktop notification is posted, once

#### Scenario: Recovery is silent

- **WHEN** a delivery confirms after a degradation was raised
- **THEN** the failure counter resets and no further degradation alert is emitted until
  the limit is reached again

### Requirement: The HQ pane is resolved robustly, and a miss queues rather than drops

The live HQ pane SHALL be resolved by a single shared rule applied by every wake call
site, accepting a pane that names the HQ home in ANY of: a pane option stamped by
`gtmux hq` at spawn (which survives a `cd`), its `pane_current_path`, or its
`pane_start_path` — with both sides of every comparison normalized through symlink
resolution, so a symlinked config directory or a temp-dir alias cannot make an HQ
invisible. A path that cannot be normalized SHALL compare raw (the rule may only add
matches, never remove one). The stamp SHALL carry the HQ home it serves rather than a
bare role flag, so a tmux server shared by two gtmux installs resolves each to its own
supervisor and never to the other's. A successful resolution SHALL be stamped; when
resolution finds no HQ but one was stamped within the last 2 hours, the wake SHALL be
enqueued for a later drain instead of discarded.

#### Scenario: A symlinked HQ home still resolves

- **WHEN** the HQ home path or the pane's reported current path differs only by a symlink
- **THEN** the pane is recognized as the HQ pane and the wake is delivered

#### Scenario: Another install's supervisor is not ours

- **WHEN** a pane on the same tmux server is stamped with a DIFFERENT gtmux install's
  HQ home
- **THEN** it does not resolve as this install's HQ pane and receives no wake

#### Scenario: A momentarily unresolvable HQ does not lose the wake

- **WHEN** a wake fires while no HQ pane resolves, but an HQ was resolved within the last
  2 hours
- **THEN** the wake is queued and delivered on the next drain that finds the HQ pane

### Requirement: Wake queue is prioritized and bounded

Queue entries SHALL carry the priority of their wake class: decision-dense classes
(`waiting`, `asks`, `goal-changed`, `crash`, `feed-degraded`, `wake-degraded`) outrank
outcome classes (`done`, `resolved`, `new-session`, `reap-suggest`, `tick`), which
outrank standing warnings (`resource·warn`, `limits·warn`). A drain SHALL emit entries
highest-priority first and oldest-first within a priority, SHALL bound one coalesced
delivery by BOTH a line count (8) and a payload size (~800 chars — large enough to be
useful, small enough that an agent TUI renders it rather than folding it into a
paste placeholder the ack could not read), and SHALL indicate that entries were held
back. The queue SHALL
be bounded (200 entries), evicting the lowest-priority oldest entry when full — a
standing warning that will re-fire is preferred over a decision-dense wake that will not.
Entries written by an earlier version SHALL remain drainable.

#### Scenario: A decision-dense wake overtakes standing warnings

- **WHEN** a `goal-changed` wake is queued behind several `resource·warn` entries
- **THEN** the next drain delivers the `goal-changed` line first

#### Scenario: A backlog cannot become one unbounded paste

- **WHEN** more than 8 entries are due in one drain
- **THEN** at most 8 lines are coalesced into the delivery, it indicates the remainder is
  queued, and the rest are delivered by the following drain

### Requirement: A queued wake is flushed within seconds

A wake queued behind a half-typed HQ draft (or a pane in copy-mode) SHALL be flushed by a
dedicated fast drain no slower than every 3 seconds while `gtmux serve` runs — not on the
resource-sampling cadence. The drain SHALL stay gated on the cheap pending-queue check,
so an empty queue costs no pane capture.

#### Scenario: A knock behind a draft lands promptly

- **WHEN** the user finishes typing in the HQ pane, leaving the box empty with a wake
  queued
- **THEN** the queued wake is delivered within ~3 seconds, without waiting for the
  resource tick

#### Scenario: A quiet queue costs nothing

- **WHEN** the fast drain fires with an empty queue
- **THEN** no pane is captured and no tmux command is run

### Requirement: A wake fires on a prompt fingerprint with an expiry, and a slash command is a user act

A prompt submitted directly into a non-HQ pane SHALL wake HQ with `goal-changed`,
deduplicated on a fingerprint of the FULL cleaned prompt with a 5-minute expiry — an
identical prompt submitted again after the window SHALL wake again (repeating an
instruction is a real event; a permanent dedup is a lost one). The pane's goal (read back
by the `done` wake) SHALL be recorded separately from the dedup fingerprint, so the
expiry cannot churn it.

A submission the user made that carries no prose — a slash command — SHALL still wake,
labelled as DATA (`goal:"(slash-command) /compact"`). Only content the user did not
author — harness-injected blocks, gtmux's own `» gtmux·` wake lines echoed back — SHALL
be silent.

#### Scenario: The same instruction, twice, an hour apart

- **WHEN** the user types the same prompt into an agent pane again after the dedup window
- **THEN** a second `goal-changed` wake is delivered

#### Scenario: A duplicate submission inside the window is suppressed

- **WHEN** an identical prompt fingerprint is seen for the same pane within 5 minutes
- **THEN** no additional wake is delivered

#### Scenario: A slash command is a user act

- **WHEN** the user runs a slash command in an agent pane
- **THEN** HQ receives a `goal-changed` wake whose goal is labelled as a slash command

#### Scenario: gtmux's own wake line never reads back as a goal

- **WHEN** a submission consists of injected harness content or a `» gtmux·` wake line
  echoed back
- **THEN** no wake is delivered

### Requirement: A stuck-before-running pane wakes as waiting, not done

The wake protocol SHALL NOT fire a `done` wake for a pane that is idle only because it is
blocked at a startup/permission gate or holds a tracked dispatch's undelivered draft, and
SHALL instead fire a `waiting` wake (kind `startup` / `draft`) so a supervisor is nudged
to unblock it. The `waiting` marker driving that wake SHALL be written by the single
writer (the serve slow-tick), never as a side effect of a read-side radar scan.

#### Scenario: Startup-gate idle fires waiting, not done

- **WHEN** the slow-tick finds a tracked dispatch pane blocked at a startup gate (or
  holding its undelivered draft)
- **THEN** it fires a `» gtmux·waiting` signal (not `» gtmux·done`) and records the
  waiting marker so the watchdog escalates the stuck worker

#### Scenario: An incidental Stop does not relabel a stuck pane done

- **WHEN** a `Stop` fires on a pane whose post-Stop screen is a startup gate or a draft
  still holding the payload
- **THEN** no `done` wake is emitted for it

