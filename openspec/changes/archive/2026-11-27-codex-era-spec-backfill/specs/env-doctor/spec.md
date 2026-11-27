## ADDED Requirements

### Requirement: Active terminal input spool capacity and occupancy diagnostics

The terminal input spool SHALL permit up to 8,192 effective active slots with an independent 8,192-name scan bound so preserved legacy queues do not block new input. Saturation SHALL refuse new input while retaining space for admitted requests and preserving pending, uncertain, and stable-ID records. `gtmux doctor --no-prompt` SHALL report active reservations and directory entry count, warning at 80 percent occupancy without authorizing record deletion.

#### Scenario: Legacy backlog coexists with new input
- **WHEN** an existing queue holds preserved legacy entries and active reservations (including claim and receipt space for unresolved indexed requests) below 8,192 effective slots
- **THEN** new terminal input is admitted without deleting pending, uncertain, or stable-ID records
- **AND** the directory scan bound remains independently capped at 8,192 names

#### Scenario: Input spool approaches or reaches slot capacity
- **WHEN** `gtmux doctor --no-prompt` inspects an input spool at or above 80 percent of the 8,192-slot limit
- **THEN** Doctor reports both active slot reservations and directory entry count with a warning without claiming a high count authorizes removal
- **AND** at saturation, new input is refused while already-admitted requests retain settlement headroom

### Requirement: Prepared storage fence conflicts surface specific layout validation errors

When a prepared storage-layout fence conflicts with an existing layout marker, `gtmux doctor` SHALL surface the specific layout validation error instead of collapsing it into generic service-preparation guidance. When preparation is valid with no readback error, Doctor SHALL keep its normal guidance to run `gtmux doctor --fix` before opening tmux to continue or cancel the prepared move.

#### Scenario: Prepared fence conflicts with a layout marker
- **WHEN** `gtmux doctor` inspects a storage root where a prepared service fence conflicts with a layout marker
- **THEN** the Storage check reports the specific layout validation error rather than normal service-preparation guidance

#### Scenario: Valid prepared service fence without conflict
- **WHEN** `gtmux doctor` inspects a validated prepared service fence with no validation error
- **THEN** Doctor reports that service recovery is preparing and instructs the operator to run `gtmux doctor --fix` before opening tmux

### Requirement: Read-only safe recovery guidance for unverified Codex bindings

When `gtmux doctor` cannot verify a Codex conversation binding, it SHALL remain strictly read-only, scope recovery guidance to panes currently running Codex, and show up to 8 `gtmux focus %N` targets. Guidance SHALL instruct the operator to preserve drafts, wait for any active turn to finish, and resume the conversation using its original launcher and Codex home while leaving the conversation and saved references untouched.

#### Scenario: Current Codex pane has an unverified conversation link
- **WHEN** `gtmux doctor` encounters a pane currently running Codex whose conversation binding is unverified or missing a resume record
- **THEN** Doctor emits read-only recovery steps including `gtmux focus %N` (bounded to 8 panes), draft preservation, and resuming with the original launcher and Codex home (such as `gtmux codex -- resume` in that pane)
- **AND** Doctor does not mutate the conversation, bindings, or saved references

#### Scenario: Retained Codex index belongs to a pane no longer running Codex
- **WHEN** a pane holds a retained Codex index while another agent is currently running in that pane
- **THEN** Doctor does not emit Codex-specific resume commands scoped to that non-Codex agent

### Requirement: Release and Store delivery outputs stay outside managed storage

Release and App Store delivery tooling SHALL verify at the actual write sites that delivery output directories and files reside outside managed `gtmux` configuration and state directories, refusing writes into managed storage while keeping project-owned archive review read-only.

#### Scenario: Delivery capture targets managed configuration or state
- **WHEN** release or Store asset capture is invoked with an output destination inside managed `gtmux` config or state
- **THEN** the write site rejects the path before creating or modifying delivery files
