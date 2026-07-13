# resource-watch Specification

## MODIFIED Requirements

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
