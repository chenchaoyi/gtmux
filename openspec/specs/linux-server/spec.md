# linux-server Specification

## Purpose
Support Linux hosts through the shared tmux/HQ core and independently testable platform adapters, with honest capability and diagnostic behavior.

## Requirements

### Requirement: Linux server workflow
The server SHALL support Linux amd64 and arm64 without cgo, including tmux discovery/control, HQ, agent hooks, installation/update and authenticated Web/mobile access.

#### Scenario: Headless operation
- **WHEN** a Linux user starts gtmux with access to their tmux socket
- **THEN** sessions are manageable without an Apple desktop or menu-bar application

### Requirement: Platform isolation
Platform-specific service management, host sampling and desktop actions SHALL be implemented in independent adapters. Shared features SHALL consume those contracts rather than duplicate per-platform branches.

#### Scenario: Background service unavailable
- **WHEN** systemd's user manager cannot be reached
- **THEN** background setup fails with actionable guidance before mutating remote access, and foreground operation remains available

### Requirement: Honest host presentation
Shared interfaces SHALL refer to the connected host without assuming macOS. Unsupported features SHALL be distinguished from errors and shall not be offered as repairs.

#### Scenario: Linux resources
- **WHEN** a Linux sample has no battery monitor
- **THEN** battery is not applicable and a complete disk/memory/load sample can clear an alarm
- **AND** failure to read an applicable source cannot clear an alarm
