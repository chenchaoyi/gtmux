## ADDED Requirements

### Requirement: Doctor validates and repairs every existing hook integration

Doctor SHALL check configuration for all eight existing hook-equipped agents found on the machine. Checks SHALL validate the executable command's agent/event binding, expected matcher, enabled state, required format and plugin template rather than merely an event name. Retired gtmux registrations SHALL require upgrade. Doctor --fix SHALL offer the corresponding existing installer, back up files, preserve foreign hooks and Kimi bytes outside the managed block, and verify the resulting configuration before reporting success. Invalid JSON and unclear managed-block boundaries SHALL refuse mutation. Global disable settings SHALL be reported and preserved. Configured hooks SHALL NOT be claimed to be loaded or trusted in existing sessions, and doctor SHALL NOT restart them.

#### Scenario: Wrong or disabled handler with the right event name

- **WHEN** a configured event invokes the wrong gtmux token, is disabled, or is scoped differently from the required binding
- **THEN** doctor recommends repair rather than reporting a complete installation
- **AND** repair replaces only gtmux's binding and verifies it

#### Scenario: User disables the hook system

- **WHEN** Claude Code disables all hooks or Gemini hooksConfig disables the hook system or a gtmux handler
- **THEN** doctor reports the disabled setting and leaves that choice unchanged

#### Scenario: Configured versus loaded

- **WHEN** a repair updates Codex hooks or an existing agent's hook configuration
- **THEN** doctor says the agent must load the changes and Codex hooks require review/trust
- **AND** it does not restart a session or bypass trust
