# supervisor-agent (delta)

## ADDED Requirements

### Requirement: The playbook teaches unattended self-rotation

The seeded playbook SHALL teach HQ that its OWN session degrades, that the degradation is
not something more care can prevent, and that recovering from it is HQ's job and not the
user's.

It SHALL name the concrete failure this rule exists for: in a long, near-full session HQ can
read its OWN prior output as input that arrived from outside, and the event stream is the
arbiter — `UserPromptSubmit` on the HQ pane is the user; `Stop` on the HQ pane is HQ itself.
Two records that read identically as text are different acts, and only the event type
distinguishes them.

On a `self-rotate` wake the playbook SHALL prescribe three steps, in order, with the user
NOT in the loop for any of them:

1. **Make the handoff durable first** — bring the situation board and the knowledge base
   current, because they are the successor session's entire briefing; anything not written
   there does not survive.
2. **Hand off** — record in the board what is in flight, what is owed, and what the
   successor must not re-derive.
3. **Rotate** — `gtmux hq --rotate`, then re-read the board before acting again.

The playbook SHALL state that a repeated `self-rotate` knock after a rotation means the
rotation did not take (the session id did not change), not that a second rotation is owed.

#### Scenario: HQ rotates without being told to

- **WHEN** a `self-rotate` wake arrives
- **THEN** HQ updates board + knowledge base, records the handoff, and runs
  `gtmux hq --rotate` — without escalating the decision to the user

#### Scenario: The playbook names the self-as-input failure

- **WHEN** a reader opens the seeded playbook
- **THEN** it explains that a `Stop` record on the HQ pane is HQ's own output and must never
  be read as the user's instruction

## MODIFIED Requirements

### Requirement: Maintenance staleness is visible in doctor

`gtmux doctor` SHALL report, in an HQ section shown only on a machine that actually has an
HQ home, the freshness of each periodic maintenance pass: `knowledge distill` against its
weekly floor and `HQ self-check` against its daily floor. A pass inside its floor SHALL read
OK; one past the floor but inside a grace window SHALL read as a neutral note (the
zero-change gate legitimately skips quiet periods); one past floor plus grace SHALL be
flagged, because the cadence itself has stopped. A pass that has never run SHALL read as an
informational "never run", not a failure.

The same section SHALL additionally report **HQ session health** — the sensed context
occupancy, session age and turn count of the supervisor's own session — flagged when a
rotation breach is standing. With no live HQ pane, or with no session resolvable for it, the
row SHALL be informational rather than a failure: an absent supervisor is not a degraded
one.

Without these rows a stalled ritual is invisible: both passes are silent by design, so "it
has not distilled in three weeks" is indistinguishable from "nothing needed distilling", and
a session too heavy to judge looks exactly like a quiet one.

#### Scenario: A stalled cadence is flagged

- **WHEN** no `self-check` pass has been raised for over a day plus its grace window
- **THEN** `gtmux doctor` flags the `HQ self-check` row with a hint to check that
  `gtmux serve` is running with a live HQ

#### Scenario: A fresh pass reads OK

- **WHEN** a `distill` pass ran within the last week
- **THEN** the `knowledge distill` row reads OK with the age of the last pass

#### Scenario: A heavy HQ session is flagged

- **WHEN** the HQ session is past a rotation threshold and has not rotated
- **THEN** the `HQ session health` row is flagged and names the breached figures

#### Scenario: No HQ running is not a health failure

- **WHEN** `gtmux doctor` runs on a machine with an HQ home but no live HQ pane
- **THEN** the `HQ session health` row is informational, not a warning
