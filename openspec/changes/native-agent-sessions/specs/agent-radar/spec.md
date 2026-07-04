## ADDED Requirements

### Requirement: Radar includes non-tmux (native) sessions
The `gtmux agents --json` payload SHALL, in addition to tmux panes, include agent sessions sensed outside tmux as rows with `source: "native"`. The addition SHALL be backward compatible: existing consumers that ignore `source` (or treat unknown sources as non-focusable) MUST continue to work, and native rows MUST NOT carry a tmux-only focusable locator.

#### Scenario: Native rows carry source and no locator
- **WHEN** a client requests `agents --json` and native sessions exist
- **THEN** each native session SHALL be a row with `source: "native"`, agent/project/state/idle-time populated, and no focusable tmux locator

#### Scenario: Backward compatibility for tmux-only clients
- **WHEN** an older client reads `agents --json` containing native rows
- **THEN** the tmux rows SHALL be unchanged in shape and the client SHALL be able to skip native rows via the `source` field without error

### Requirement: Native rows are not focus/jump targets
The radar SHALL mark native rows as neither focusable nor send-able, so surfaces do not offer jump-to-terminal or reply on them.

#### Scenario: Focus is refused for a native session
- **WHEN** a focus/jump is attempted against a native session's identity
- **THEN** the system SHALL NOT attempt a terminal jump (there is no tmux/terminal locator for it)
