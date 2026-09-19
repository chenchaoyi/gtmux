# env-doctor (delta)

## ADDED Requirements

### Requirement: Doctor can pack a bug report

`gtmux doctor --bundle <path>` SHALL write an archive of `logs/`, `status/`, the doctor
report and version information, SHALL exclude the journal and user data unless
`--with-events` is given, and SHALL print what it included.

#### Scenario: Default bundle

- **WHEN** the user runs `gtmux doctor --bundle out.tgz`
- **THEN** `out.tgz` contains no `events.jsonl` and no uploads, and the command lists the
  files it packed
