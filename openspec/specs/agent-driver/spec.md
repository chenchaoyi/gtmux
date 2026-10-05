# agent-driver Specification

## Purpose
TBD - created by archiving change agent-drivers. Update Purpose after archive.
## Requirements
### Requirement: Two-layer perception/drive model with tmux as the permanent base

The system SHALL organize agent perception and drive into two layers. Layer 1 —
the tmux base (pane lifecycle, screen capture, keystroke injection) — SHALL work
for ANY terminal agent with zero integration and SHALL be permanently retained:
no channel may remove or bypass its screen/keystroke path. Layer 2 — per-agent
drivers — SHALL be an optional set of capabilities (delivery receipt, readiness,
content, headless one-shot) resolved per agent from a single registry. A
configuration switch SHALL exist to disable drivers globally (`driver.enable`) and
per agent-capability (`driver.<agent>.<capability>`). A capability that an agent lacks,
or that is switched off, SHALL have exactly this effect, as the agent-drivers change
delivered it (archive 2026-07-24-agent-drivers, tasks 4.1/4.3/5.2, design §2.4/§2.5/§5):

- **receipt**: delivery verification falls back to the Layer 1 screen read;
- **ready**: the readiness gate falls back to the Layer 1 screen read;
- **content**: the transcript is not read, so the digest's `goal` and `last` are absent
  for that row (they are not reconstructed from the screen) and its `sense` is
  `partial` or `screen`;
- **headless**: `spawn --oneshot`, which is opt-in, is refused with a message, never
  turned into an interactive spawn.

The hook state records (the waiting/active markers the radar reads directly) are not a
driver capability: the switches do not turn them off.

#### Scenario: An agent without a driver is fully managed by Layer 1

- **WHEN** an unknown terminal agent runs in a tmux pane with no driver registered
- **THEN** radar, send, spawn, and wake behave exactly as before this change —
  screen-based classification, screen-verified delivery, screen readiness gates

#### Scenario: Turning drivers off

- **WHEN** `driver.enable` is off (or a specific `driver.<agent>.<capability>` is off)
- **THEN** delivery and readiness for the affected agents are judged from the screen;
  their digest rows carry no `goal`/`last` and a `sense` of `partial` or `screen`;
  `spawn --oneshot` for them is refused; and the hook-fed status stays as it was. No
  field is renamed or changes meaning; the values a capability supplied are absent

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
`agents --json` and `digest --json` field meanings, `gtmux tasks`/`spawn`/
`send` semantics, and the wake classes and their meanings SHALL be the same
whether a row is served by a driver or by Layer 1. Driver-related surface
changes SHALL be additive only (new optional fields, new opt-in flags). The VALUES
are not promised identical: a field a capability fills (`goal`, `last`) is absent
without it, and the additive `sense` says which tier served the row.

#### Scenario: A consumer cannot tell layers apart except by additive fields

- **WHEN** the same fleet is read with drivers enabled and disabled
- **THEN** every field keeps its name and meaning; content-fed fields (`goal`, `last`)
  may be absent with drivers disabled, and the additive `sense` annotation differs
