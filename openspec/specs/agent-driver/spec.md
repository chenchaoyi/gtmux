# agent-driver Specification

## Purpose
TBD - created by archiving change agent-drivers. Update Purpose after archive.
## Requirements
### Requirement: Two-layer perception/drive model with tmux as the permanent base

The system SHALL organize agent perception and drive into two layers. Layer 1 —
the tmux base (pane lifecycle, screen capture, keystroke injection) — SHALL work
for ANY terminal agent with zero integration and SHALL be permanently retained:
no channel may remove or bypass its screen/keystroke path. Layer 2 — per-agent
drivers — SHALL be an optional set of capabilities (delivery receipt, state
truth, content, readiness, headless one-shot) resolved per agent from a single
registry; an agent with no driver (or a driver missing a capability) SHALL fall
back to Layer 1 for that channel with behavior identical to the pre-driver
system. A configuration switch SHALL exist to disable drivers globally
(`driver.enable`) and per agent-capability, restoring Layer 1 behavior
byte-for-byte.

#### Scenario: An agent without a driver is fully managed by Layer 1

- **WHEN** an unknown terminal agent runs in a tmux pane with no driver registered
- **THEN** radar, send, spawn, and wake behave exactly as before this change —
  screen-based classification, screen-verified delivery, screen readiness gates

#### Scenario: Disabling drivers restores baseline behavior

- **WHEN** `driver.enable` is off (or a specific `driver.<agent>.<capability>` is off)
- **THEN** every affected channel behaves identically to the pre-driver system,
  with no schema or semantic change visible to any consumer

### Requirement: Driver evidence is positive-monotonic

Driver-grade evidence SHALL be used only to CONFIRM success (a landing, a
readiness, a completion) and SHALL NEVER by its absence be treated as proof of
failure — a missing driver signal only means the judgment falls to Layer 1.
Conversely, screen-read evidence SHALL NOT overturn a driver-confirmed success:
once the driver confirms, the verdict is final. Before any channel declares a
FAILURE (e.g. `delivered:false`), it SHALL perform a final re-check of the
driver evidence source, so a structured confirmation that arrived late is never
lost to a screen-read timeout.

#### Scenario: A late driver confirmation beats a screen timeout

- **WHEN** a delivery's screen verification is about to time out as failed while
  the driver's event stream by then contains a matching submit confirmation
- **THEN** the final re-check finds the confirmation and the delivery is
  reported landed, not failed

#### Scenario: Absent driver evidence is not failure

- **WHEN** a driver-capable agent produces no relevant event within the grace
  window
- **THEN** the channel proceeds with the Layer 1 (screen) judgment, and the
  missing event alone never yields a failure verdict

### Requirement: Drivers consume produced facts only, never wrap the agent

A driver SHALL derive its evidence exclusively from facts the agent already
produces (hook event streams, transcript files, on-disk state markers,
non-interactive exec output). The system SHALL NOT insert a resident proxy
process, PTY middleman, or persistent programmatic session between the user and
an interactive agent — the tmux pane remains the single input path, so the user
can always jump in and take over.

#### Scenario: The user takes over a driver-managed session

- **WHEN** the user attaches to a pane whose agent has a full driver
- **THEN** they interact with the agent TUI directly, with no gtmux process
  between their keystrokes and the agent

### Requirement: The external model is driver-agnostic

Driver upgrades SHALL NOT change any existing external contract: the
`agents --json` and `digest --json` field semantics, `gtmux tasks`/`spawn`/
`send` semantics, and the wake classes and their meanings SHALL be identical
whether a row is served by a driver or by Layer 1. Driver-related surface
changes SHALL be additive only (new optional fields, new opt-in flags).

#### Scenario: A consumer cannot tell layers apart except by additive fields

- **WHEN** the same fleet is read with drivers enabled and disabled
- **THEN** all pre-existing fields and semantics are identical; only additive
  fields (e.g. the digest `sense` annotation) may differ
