## MODIFIED Requirements
### Requirement: Machine resource snapshot

The system SHALL compute a deterministic, cgo-free snapshot of local resources:
on macOS: disk free and capacity% (via `df` on the WRITABLE data volume — macOS
`/System/Volumes/Data`, falling back to `/` where that is absent, so capacity%
reflects real user usage and not the near-empty read-only system volume), memory
pressure (via `sysctl -n kern.memorystatus_vm_pressure_level`, mapping
1/2/4 to normal/warn/critical; free percentage is sampled separately with
`memory_pressure -Q`), CPU saturation (loadavg ÷ core count), and POWER/BATTERY (via `pmset -g
batt` on macOS: charge %, on-AC vs draining, state, and time-left; `present:false` on a
battery-less host, and the battery object omitted when the command fails, its answer has no
power-source line, or it has a battery line whose charge cannot be read). A source
that is unavailable SHALL degrade to an empty field
without failing the rest.

On Linux, the sampler SHALL use statfs on the gtmux-state filesystem, MemAvailable / MemTotal from procfs (warn below 10%, critical below 5%), and the 1-minute load average divided by accessible CPUs. Battery monitoring is not applicable for this server implementation. Completeness SHALL distinguish a not-applicable battery source from a failed applicable source; missing disk/memory/load readings SHALL NOT clear an existing alarm.

The snapshot SHALL also expose an overall severity `tier`
(`amber` | `red`; omitted when normal) — the worst of the disk/memory/load/battery
tiers — so a consumer can distinguish a soft heads-up from a genuine bottleneck
without re-deriving thresholds. The snapshot's `warn` sentence SHALL be in the
process's language (`GTMUX_LANG`), and the same condition SHALL also be named as a key
(`warn_key`: `disk-low` | `disk-critical` | `memory-warn` | `memory-critical` |
`load-high` | `load-critical` | `battery-low` | `battery-critical`), so a consumer that
speaks another language than the serve (the phone, whose launchd serve has none) words
it from the key and the readings. A LOW battery charge SHALL count toward the warn/tier
ONLY while the machine is drawing from the battery (never on AC), so a plugged-in
laptop is never flagged for its charge level. An exhausted reading SHALL be judged as the
worst tier, never as a missing one: `df` answering 0 free with its capacity is a full
volume, and a charge read as 0% while draining is an empty battery. Only a source that
did not answer is missing.

#### Scenario: Snapshot reflects the machine

- **WHEN** `gtmux resource` is run
- **THEN** it reports disk free + capacity% on the writable data volume, the
  memory-pressure tier, load ÷ cores, the battery (charge % · on-AC/draining · time
  left) when present, and an overall `tier` when not normal

#### Scenario: A draining low battery warns, on AC it does not

- **WHEN** the machine is on battery power and the charge falls below the battery
  amber/red threshold
- **THEN** the snapshot's `warn` names the battery and the overall `tier` reflects it
  (so it rides the existing `resource·warn` nudge to HQ)
- **AND WHEN** the same low charge is seen while plugged into AC
- **THEN** the battery contributes nothing to `warn`/`tier`

#### Scenario: A full disk and an empty battery are readings

- **WHEN** `df` reports 0 free with a capacity, or the battery reads 0% while draining
- **THEN** that resource is at its red tier and the snapshot warns
- **AND WHEN** `df` or `pmset` does not answer at all
- **THEN** that resource contributes nothing, rather than reading as full or empty

#### Scenario: Linux resource completeness
- **WHEN** Linux provides valid disk, available-memory and load readings
- **THEN** the sample is complete without a battery monitor
- **AND** a missing applicable reading prevents alarm recovery
