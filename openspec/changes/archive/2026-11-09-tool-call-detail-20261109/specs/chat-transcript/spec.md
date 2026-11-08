## ADDED Requirements

### Requirement: Read recorded native tool details on demand

The system SHALL preserve bounded selected inputs in ordinary transcript polls and expose only bounded metadata for recorded Codex native MCP/command completions. Their arguments/results SHALL be recoverable through the existing authenticated owner/full-paired exact-record reader, without widening guest access or retaining result bodies in transcript caches. Structured credential fields and binary MCP blocks SHALL be filtered and identified as filtered.

#### Scenario: Service query includes parameters and a recorded result

- **WHEN** a native service call records repository/hash parameters and structured or textual output
- **THEN** App and Web Chat/Trace use shared analysis to show the actual tool, provider, recorded status/duration, named parameters and the recorded result
- **AND** Preview and Raw are useful even when no command/path field was selected
- **AND** missing, empty, error, loading and failed-read states remain distinct

#### Scenario: Native completion mirrors an earlier invocation

- **WHEN** the exact call ID and normalized tool identify an earlier invocation
- **THEN** it remains one chronological call with the original input/ref intact and a separate exact native detail reference
- **AND** unrelated IDs remain distinct, and stale or foreign-session references cannot select another record

#### Scenario: Large result is opened

- **WHEN** a recorded detail fits the existing 32 MiB source-record bound but is too large for a compact reader
- **THEN** the visible content is explicitly paged and complete section copying remains available
- **AND** oversized, incomplete or rewritten records fail explicitly rather than appearing complete
