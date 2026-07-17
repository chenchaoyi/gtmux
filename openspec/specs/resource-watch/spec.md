# resource-watch Specification

## Purpose
TBD - created by archiving change resource-watch. Update Purpose after archive.
## Requirements
### Requirement: Machine resource snapshot

The system SHALL compute a deterministic, cgo-free snapshot of local resources:
disk free (via `df` on the relevant volume), memory pressure (via
`memory_pressure -Q`, mapping its normal/warn/critical to the warn tiers), and CPU
saturation (loadavg ÷ core count). A source that is unavailable SHALL degrade to
an empty field without failing the rest.

#### Scenario: Snapshot reflects the machine

- **WHEN** `gtmux resource` is run
- **THEN** it reports disk free, the memory-pressure tier, and load ÷ cores

### Requirement: Per-agent resource attribution

The system SHALL attribute resource use to specific agents by walking each radar
pane's process tree from its pane PID and summing RSS and CPU%, surfaced per
digest/usage row — so HQ can see which agent is heavy, isomorphic to token
accounting.

#### Scenario: Heavy agent is identifiable

- **WHEN** an agent's process tree consumes significant RSS/CPU
- **THEN** its digest/usage row carries that RSS/CPU

### Requirement: Actionable reclaim candidates

The system SHALL identify reclaimable processes — heavy processes NOT owned by any
live pane (orphans a prior session left behind, e.g. a leftover simulator runtime
or a still-listening dev server) — each named with its pid and a reclaim hint, so
HQ's advice is executable rather than vague.

#### Scenario: Orphan named for reclaim

- **WHEN** a heavy process is not under any live pane's tree
- **THEN** it appears as a reclaim candidate with pid + hint

### Requirement: Tick-driven warnings with correct dedup

The system SHALL evaluate resource tiers on the serve tick and emit a
`resource·warn` nudge to a live HQ ONLY from that single-writer tick — never from
a getter invoked by multiple concurrent callers — so a single crossing is nudged
exactly once. Dedup SHALL key on the TIER (normal/amber/red), NOT the exact warning
value: a value that jitters WITHIN the same tier (e.g. disk-free 40→39→38 GB, all
amber) SHALL NOT re-nudge; only a tier crossing nudges. The same single-writer,
by-tier dedup SHALL apply to `limits·warn`.

A tier crossing SHALL additionally be damped against a value dithering ON a threshold,
by three mechanisms:

- **Hysteresis.** A tier SHALL be entered at its configured threshold but left only once
  the sample clears an exit margin (`resource.diskHysteresisGB`, default 2 GB;
  `resource.loadHysteresis`, default 0.15) — e.g. red at under 15 GB free clears only at
  17 GB or more. Memory, whose tier is the kernel's already-discrete pressure level, needs
  no margin. The reported snapshot (`gtmux resource`, digest, `GET /api/usage`) SHALL
  keep reporting the RAW tier: hysteresis governs the alert, not the readout.
- **Confirmation window.** A tier change SHALL commit only after `resource.confirmSamples`
  (default 3) consecutive samples agree on it.
- **Minimum restate interval.** A committed tier SHALL NOT re-nudge within
  `resource.minRestateMinutes` (default 30) of the last nudge — UNLESS it is an
  escalation to a strictly more severe tier, which SHALL always nudge.

`limits·warn`, whose dedup key is a window identity rather than an ordered severity,
SHALL keep the plain by-tier dedup: suppressing a new window's first warning would be a
loss, not a damped flap.

#### Scenario: One crossing, one nudge

- **WHEN** a resource crosses into a warn tier while HQ is live
- **THEN** exactly one `resource·warn` line is delivered

#### Scenario: Intra-tier jitter does not re-nudge

- **WHEN** a resource value changes but stays within the same tier (e.g. disk-free
  drifts 40→39→38 GB, all amber)
- **THEN** no additional nudge is delivered until the tier itself changes

#### Scenario: A value dithering on the threshold does not flap

- **WHEN** disk-free oscillates across the red line (15.1 → 14.9 → 15.1 GB) and load
  oscillates around 1.0× cores
- **THEN** the tier holds until the sample clears the exit margin, and no repeated
  `resource·warn` is delivered

#### Scenario: A brief spike does not commit a tier

- **WHEN** a single sample reads a worse tier and the next samples do not agree
- **THEN** no tier change commits and no nudge is delivered

#### Scenario: An escalation is never suppressed by the restate interval

- **WHEN** a confirmed amber escalates to red within the minimum restate interval
- **THEN** the `resource·warn` for red is delivered immediately
### Requirement: Resource surfaces and pre-flight check

The system SHALL expose `gtmux resource [--json]` and include a resource block on
`GET /api/usage`/digest (snapshot + per-agent + reclaim candidates). Before adding
load — `gtmux hq`/`gtmux new` — the system SHALL warn when a resource is at its red
line.

#### Scenario: Pre-flight red-line warning

- **WHEN** the user spawns a session while a resource is at its red line
- **THEN** the command warns before proceeding

