# env-doctor (delta)

## ADDED Requirements

### Requirement: doctor reports knowledge distribution per agent

`gtmux doctor` SHALL show a "knowledge sync" row per supported agent with one of: in
sync, missing, stale, hand-edited. `--fix` SHALL refresh missing and stale blocks and
SHALL leave hand-edited ones with a message.

#### Scenario: A new agent is installed after the last sync

- **WHEN** an agent's global instruction file exists without the block
- **THEN** doctor reports it missing and `--fix` installs the block
