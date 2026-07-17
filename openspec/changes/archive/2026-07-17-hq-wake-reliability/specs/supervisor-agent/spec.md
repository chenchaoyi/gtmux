# supervisor-agent — delta

## ADDED Requirements

### Requirement: The playbook teaches the wake re-send identifier

The seeded playbook SHALL teach that every wake line ends with a short `#<id>` batch
identifier, and that a line whose identifier HQ has already acted on is a RE-SEND of an
unconfirmed delivery — to be ignored, not treated as a second event. It SHALL likewise
teach the `(slash-command)` goal payload (a user act with no prose, not an agent
message) and the `wake-degraded` class (the wake channel itself is failing to confirm —
reconcile by pull rather than trusting the knock). The shipped playbook version SHALL be
bumped so existing homes receive these conventions on the next `gtmux hq` after an
update.

#### Scenario: A duplicated wake is recognized

- **WHEN** HQ receives two wake lines carrying the same trailing `#<id>`
- **THEN** the playbook has it treat the second as a re-send and take no second action

#### Scenario: Existing homes get the conventions

- **WHEN** `gtmux hq` runs against a home carrying the previous managed playbook version
- **THEN** the playbook is regenerated at the new version (the prior file backed up) and
  states the `#<id>`, `(slash-command)`, and `wake-degraded` conventions

## MODIFIED Requirements

### Requirement: Dual-channel dispatch — HQ senses user-direct tasks

The system SHALL let HQ track work the user dispatches through EITHER channel: via HQ
(`gtmux spawn`, tracked) or by typing directly into an agent's own window. When a
`UserPromptSubmit` occurs in a pane that is NOT the HQ pane, the system SHALL push a
`goal-changed` nudge to a live HQ carrying the pane and the prompt head (as DATA), gated
on a live HQ pane and `hqNudge`, and never about HQ's own prompts.

The nudge SHALL be deduplicated per pane on a FINGERPRINT of the full cleaned prompt
carrying a timestamp, suppressing only an identical prompt within a 5-minute window — a
resubmit of the same prompt inside the window does not spam, and the same instruction
repeated after it wakes HQ again. The pane's goal that the `done` wake reads back SHALL
be recorded separately from that dedup fingerprint, so the expiry cannot churn it.

A submission with no prose that the user nonetheless made — a slash command — SHALL
still wake, with its goal labelled as DATA (`goal:"(slash-command) /compact"`); only
content the user did not author (harness-injected blocks, gtmux's own wake lines echoed
back) SHALL be silent.

The HQ playbook SHALL instruct that observing an agent working on a task NOT in the
ledger, the FIRST assumption is the user dispatched it directly — HQ verifies (records it
as `user-direct`) rather than "correcting", interrupting, or overwriting it.

#### Scenario: A user-direct prompt reaches HQ

- **WHEN** the user submits a prompt directly in a non-HQ agent pane and an HQ pane
  is live
- **THEN** HQ receives one `goal-changed` nudge for that pane (deduped per prompt
  fingerprint within the window)

#### Scenario: The same instruction after the window wakes HQ again

- **WHEN** the user submits the same prompt into the same pane after the dedup window has
  expired
- **THEN** a second `goal-changed` nudge is delivered

#### Scenario: Off-ledger work is presumed user-direct

- **WHEN** HQ observes an agent working on a task not in its ledger
- **THEN** the playbook has HQ presume it is user-direct and verify, not correct it
