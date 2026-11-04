## ADDED Requirements

### Requirement: Codex recovery requires durable same-home history
Live frontend identity SHALL remain separate from its last verified recovery checkpoint.
Preview and execution SHALL inspect matching regular rollout metadata in the selected
Codex home, including legacy IDs, archived files, continuation rollouts and `.jsonl.zst` history.
Plain siblings SHALL take precedence over compressed copies. An allocated
unsaved ID SHALL not authorize resume, and an older checkpoint SHALL not silently replace
the current thread. Missing, unreadable, malformed and foreign-home history SHALL remain
distinct. Paginated history SHALL not require parsing mutable SQLite schemas.

#### Scenario: HQ switched to an unsaved thread before reboot
- **WHEN** a saved frontend names an in-memory thread without durable history
- **THEN** restore skips its resume command and explains why
- **AND** a prior verified checkpoint remains distinct from the live thread

#### Scenario: Older or archived Codex history remains usable
- **WHEN** readable metadata has the exact target ID in legacy, archived or continued history
- **THEN** both preview and restore recognize the same recovery point
- **AND** a foreign home or mismatching header never supplies that proof

### Requirement: Restore receipts distinguish submission and startup
Normal terminal output SHALL use aligned, display-width-bounded pane rows and workspace
totals, preserving complete IDs at narrow widths. A successful key send SHALL be recorded
as command submission, not recovered conversation success. Fresh same-home target binding
or hook evidence SHALL establish a separate startup confirmation; otherwise the result
SHALL remain pending. Missing/invalid history and send failures SHALL be explicit.

#### Scenario: A client rejects the command after tmux accepted it
- **WHEN** the command is sent but no fresh target confirmation follows
- **THEN** the output and diagnostics retain pending startup rather than confirmed recovery

#### Scenario: Many windows restored into a narrow terminal
- **WHEN** the pane receipt contains long names or IDs
- **THEN** rows align and wrap within available columns without dropping identities or interpreting control characters
