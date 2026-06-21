# Session Restore Specification

## Purpose

Bring your tmux workspace back after the terminal quits or the machine reboots —
layout, directories, and screen text — driving tmux-resurrect/continuum
deterministically so a large layout is never lost or silently replaced.

## Requirements

### Requirement: Reattach after quitting the terminal

The system SHALL, when the tmux server is still alive but the terminal was quit,
open one terminal tab per session and attach them (see terminal-jump's restore).

#### Scenario: Tabs gone, sessions alive

- **WHEN** the terminal is quit and `gtmux restore` is run
- **THEN** each tmux session gets an attached terminal tab

### Requirement: Restore after reboot via resurrect

The system SHALL, when the tmux server is down, start it and DRIVE tmux-resurrect
restore explicitly (run-shell, in-server context), waiting for the restore to
complete rather than racing a fixed timeout.

#### Scenario: Large layout takes time

- **WHEN** restoring a large saved layout that takes longer than a fixed timeout
- **THEN** the system waits until restored sessions settle, then proceeds

### Requirement: Never overwrite a good save

The system SHALL, if a saved layout exists but did not restore, refuse to keep a
bare fallback session as if all is well (which continuum would then autosave over
the good save) — it warns and points at the save instead.

#### Scenario: Restore failed but a save exists

- **WHEN** a save with real sessions exists but the restore produced nothing
- **THEN** the system warns loudly, does not overwrite the save, and surfaces its
  path for recovery

### Requirement: Repair a poisoned `last` pointer

The system SHALL, before booting the server, repair a tmux-resurrect `last`
symlink that points at an empty save (resurrect repoints `last` on any content
change, even a 0-byte race), pointing it at the newest save that has a layout.

#### Scenario: `last` points at an empty save

- **WHEN** `last` is missing or resolves to a save with no window/pane lines
- **THEN** it is repointed to the newest timestamped save that has a layout; a
  `last` already resolving to a real layout is left untouched

### Requirement: Restore scrollback when configured

The system SHALL restore each pane's scrollback (screen text) when
`@resurrect-capture-pane-contents` is on, with `history-limit` controlling depth.

#### Scenario: Scrollback snapshot

- **WHEN** capture-pane-contents is enabled and a restore runs
- **THEN** each pane's prior scrollback comes back as a snapshot
