# hq-wake-protocol — delta

## ADDED Requirements

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
