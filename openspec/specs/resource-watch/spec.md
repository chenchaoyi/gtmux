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

#### Scenario: One crossing, one nudge

- **WHEN** a resource crosses into a warn tier while HQ is live
- **THEN** exactly one `resource·warn` line is delivered

#### Scenario: Intra-tier jitter does not re-nudge

- **WHEN** a resource value changes but stays within the same tier (e.g. disk-free
  drifts 40→39→38 GB, all amber)
- **THEN** no additional nudge is delivered until the tier itself changes

### Requirement: Resource surfaces and pre-flight check

The system SHALL expose `gtmux resource [--json]` and include a resource block on
`GET /api/usage`/digest (snapshot + per-agent + reclaim candidates). Before adding
load — `gtmux hq`/`gtmux new` — the system SHALL warn when a resource is at its red
line.

#### Scenario: Pre-flight red-line warning

- **WHEN** the user spawns a session while a resource is at its red line
- **THEN** the command warns before proceeding

