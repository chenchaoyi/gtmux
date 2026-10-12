## ADDED Requirements

### Requirement: Codex history retention is independent of tool log volume
Codex conversation loading SHALL retain the most recent parsed turns up to the
existing cache limit, even when intervening non-chat records exceed the raw tail
byte limit. Non-chat records SHALL NOT consume the conversation retention budget.
The loader SHALL preserve ordered prompts and public assistant segments without
changing the original log. Existing HTTP turn and payload limits still apply.

#### Scenario: Tool output exceeds the raw tail limit
- **WHEN** two user turns are separated from the file end by more than 64 MiB of tool output
- **THEN** both turns remain available from the transcript loader in order
- **AND** collapsing their replies does not combine the turns

#### Scenario: A long active turn grows
- **WHEN** new complete records are appended to a cached Codex rollout
- **THEN** the loader reads from its last complete-record offset
- **AND** updates the active turn without mutating previously returned snapshots

#### Scenario: Partial record and log replacement
- **WHEN** the last record is only partially written
- **THEN** it remains pending and is parsed once after completion
- **WHEN** the log shrinks, is replaced or is rewritten at the same size
- **THEN** the cache is rebuilt from the current file
