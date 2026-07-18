# supervisor-agent (delta)

## ADDED Requirements

### Requirement: gtmux raises a periodic knowledge-distillation trigger

The system SHALL raise a `distill` trigger to a live HQ when a knowledge-distillation
pass is due, decided LLM-free in the serve slow-tick — no LLM runs in the timing loop;
HQ performs the actual curation on the control record (the same split as the self-check
sensor). The trigger SHALL be gated by a coarse cadence: a rate limit so it can never
fire more than once per configured minimum interval, then an EVENT-VOLUME floor (a pass
is due once enough new events have accrued since the last distill — set so a busy fleet
distills before its size-bounded event log rotates the delta away) OR a TIME floor (a
weekly pass regardless), and a ZERO-CHANGE gate (when nothing notable accrued since the
last distill, no trigger is raised and no cost is incurred). Each raised trigger SHALL
advance a `last-distill` watermark (an event sequence / timestamp marker) so the next
pass distills only the DELTA since this one. When no HQ pane exists, no trigger SHALL be
raised.

#### Scenario: A weekly pass is due

- **WHEN** the time floor has elapsed since the last distill and at least one notable
  event has accrued
- **THEN** gtmux raises exactly one `distill` control record and advances the watermark

#### Scenario: A busy fleet distills before the log rotates

- **WHEN** the event-volume floor is reached well before the weekly floor (a high-churn
  period)
- **THEN** the `distill` trigger fires on the volume floor, so the delta is distilled
  before the size-bounded event log rotates it away

#### Scenario: Nothing to distill costs nothing

- **WHEN** a cadence boundary is reached but no notable event accrued since the last
  distill
- **THEN** no `distill` trigger is raised (the zero-change gate)

#### Scenario: No trigger without a supervisor

- **WHEN** no HQ pane is present
- **THEN** no `distill` trigger is raised

### Requirement: The seeded playbook teaches the knowledge-distillation ritual

The seeded playbook SHALL teach HQ, on a gtmux-raised `distill` control record
(`[CONTROL gtmux:distill]`, delivered on the silent feed like `self-check` — a
low-urgency maintenance signal, NOT a typed wake line that interrupts the pane), to run
a retrospective knowledge-distillation pass: read the fleet's event/outcome delta since
the last distill, fold durable cross-cutting facts into the right knowledge-base topic
file (preferring to UPDATE existing entries over appending duplicates), and PRUNE stale
or dead entries and merge duplicates — using only its existing write-own-notes
authority. The ritual SHALL be distinct from `self-check` (own-artifact health
housekeeping) and `tick` (the user-facing summary brief). HQ SHALL default to SILENT
distillation, printing a one-line
brief ONLY when it made a real curation; a charter-level lesson SHALL still be flagged
for a seed/spec update rather than only noted locally; and the never-store-secrets rule
SHALL continue to apply. The shipped playbook version SHALL be bumped so existing HQ
homes adopt the ritual on their next managed-playbook upgrade.

#### Scenario: Silent when nothing durable accrued

- **WHEN** a `distill` trigger fires and the period's delta yields no durable new fact,
  no stale entry, and no duplicate to merge
- **THEN** HQ performs the pass and prints nothing

#### Scenario: A real distillation is briefed in one line

- **WHEN** a `distill` trigger fires and HQ folds a recurring cross-session fact into a
  topic file and prunes a dead entry
- **THEN** HQ prints a single one-line brief of what it curated

#### Scenario: The distill delta is not a duplicate of moment-capture

- **WHEN** a durable fact was already captured in the knowledge base the moment it was
  learned
- **THEN** the distill pass updates that entry in place rather than appending a second
  copy, because it works the delta since the watermark and consolidates rather than
  re-summarizes

#### Scenario: Secrets are never distilled into the base

- **WHEN** the period's activity includes a password, token, or private key
- **THEN** the distillation records only IDs / methods / pointers, never the secret

### Requirement: HQ verifies perception self-heal before nagging or restarting

The seeded playbook SHALL teach HQ that a `feed-degraded` or `wake-degraded` wake
reports that gtmux's OWN mechanical self-heal has ALREADY run — it is a report, not a
request for HQ to restart anything. HQ SHALL first VERIFY by pulling the live
digest/events: when perception is actually fresh, HQ SHALL stay silent (record only)
and SHALL NOT repeatedly nag the user to restart. Only when the data is genuinely
stale/broken SHALL HQ act, and per the role boundary it SHALL restart nothing itself —
it SHALL dispatch a worker to restart the feed daemon and escalate to the user. (This
charter discipline is folded into the knowledge-distillation change so the seeded
playbook version bumps once; the code-side disk/feed hardening ships separately and
touches no playbook.)

#### Scenario: Fresh perception after a degraded wake stays silent

- **WHEN** a `feed-degraded` wake arrives but a pull of `gtmux digest`/`events` shows
  perception is current (the mechanical self-heal recovered)
- **THEN** HQ records it and stays silent — it does not nag the user to restart

#### Scenario: A genuinely broken feed is restarted via a worker, not by HQ

- **WHEN** after a degraded wake the pulled digest/events are genuinely stale/broken
- **THEN** HQ dispatches a worker to restart the feed daemon and escalates to the user,
  and never runs the restart command itself
