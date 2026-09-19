# diagnostics (delta)

Phase 1 of this change is built and synced into `openspec/specs/diagnostics/spec.md`.
What remains is below.

## ADDED Requirements

### Requirement: Every act that changes something is recorded

Beyond what serve records, every act that changes something on the Mac, whoever starts
it, SHALL be recorded as an action entry with its actor: every command the command table
marks as writing (with actor `user`, `hq`, or `agent:%N` for a gtmux command run from
another agent's pane), HQ's acts, and the menu bar's. The acts the journal audits for HQ
SHALL be written to the journal and the log through one call, so the two cannot disagree.

#### Scenario: A command marked as writing has no action

- **WHEN** a command in the command table is marked as writing and the action catalog has
  no event for it
- **THEN** the test suite fails

### Requirement: Entries leave the machine only through an explicit export

Entries SHALL leave the machine only through an explicit export: `gtmux doctor --bundle`,
or the phone's share action on its own diagnostics buffer. An export SHALL exclude the
journal and user data unless the user opts the journal in.

#### Scenario: A bug report bundle

- **WHEN** the user runs `gtmux doctor --bundle report.tgz`
- **THEN** the archive holds `logs/`, `status/`, the doctor report and version information,
  no `events.jsonl` and no uploads, and the command lists what it included
