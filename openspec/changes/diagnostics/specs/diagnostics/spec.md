# diagnostics (delta)

Phases 1 and 2 of this change are built and synced into
`openspec/specs/diagnostics/spec.md`. What remains is below.

## ADDED Requirements

### Requirement: Entries leave the machine only through an explicit export

Entries SHALL leave the machine only through an explicit export: `gtmux doctor --bundle`,
or the phone's share action on its own diagnostics buffer. An export SHALL exclude the
journal and user data unless the user opts the journal in.

#### Scenario: A bug report bundle

- **WHEN** the user runs `gtmux doctor --bundle report.tgz`
- **THEN** the archive holds `logs/`, `status/`, the doctor report and version information,
  no `events.jsonl` and no uploads, and the command lists what it included
