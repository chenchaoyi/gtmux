## ADDED Requirements

### Requirement: Hook terminal outcomes preserve interruption and failure semantics

The system SHALL distinguish successful completion, user interruption and terminal failure using each agent's documented lifecycle payload. Interrupted or failed turns SHALL clear working and waiting without earning a completion notification or latest-completed pane update. Only a subsequent actual prompt or turn start SHALL reset the terminal outcome. Tool failures, recoverable errors and child-session endings SHALL NOT terminate the parent turn. Known mismatched turn identifiers SHALL NOT end a newer turn. Missing upstream events SHALL retain existing screen/transcript fallback rather than be invented.

#### Scenario: Codex interruption with a missing hook

- **WHEN** a pane's bound Codex rollout ends with turn_aborted after its active and waiting markers
- **THEN** its state becomes idle with an interrupted outcome and no successful-finish stamp
- **AND** a newer task_started, session mismatch or changed cwd prevents old reconciliation

#### Scenario: Late callback after cancellation

- **WHEN** a tool or permission callback reaches an interrupted tmux or native session before a real new turn starts
- **THEN** it cannot restore working or waiting or relabel that outcome as success

#### Scenario: Payload-specific and child outcomes

- **WHEN** Cursor stop reports aborted, Copilot reports a recoverable error, or OpenCode emits a child session ending
- **THEN** Cursor is interrupted, the recoverable error does not end the turn, and the child cannot change the root pane
- **AND** a subsequent OpenCode idle after root session.error does not emit successful completion

### Requirement: Hook artifacts follow current official formats and preserve compatibility

The system SHALL document the eight existing hook contracts and their limits in English and Chinese, including primary sources. Installers SHALL add supported state events, remove only gtmux's retired bindings, and preserve foreign hooks. Kiro SHALL support both CLI 2.x profile hooks and V3 global hooks, preserving foreign array-form profile hooks during migration. OpenCode SHALL install in the current global plugins directory and migrate only its owned legacy plugin. Observational hooks SHALL NOT make approval decisions; a tool-start callback that also runs on authorized tools SHALL NOT alone prove a permission wait.

#### Scenario: Current Kiro and OpenCode configuration migration

- **WHEN** gtmux upgrades to Kiro V3 globals or the current OpenCode plugin directory
- **THEN** it backs up and removes only its own duplicate legacy artifact or bindings
- **AND** foreign profile hooks, plugin contents and configuration remain preserved
