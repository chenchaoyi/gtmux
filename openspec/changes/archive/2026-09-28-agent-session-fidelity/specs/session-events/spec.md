# session-events Specification Delta

## ADDED Requirements

### Requirement: Codex hook events are bound only to a uniquely identified live pane

When a Codex hook carries a session id, gtmux SHALL prefer a unique live pane whose resume
binding names that session and whose agent and cwd agree. A cwd-only fallback SHALL be used
only when exactly one live Codex pane matches. If multiple panes match, gtmux SHALL leave the
pane association empty rather than write the event to a guessed pane.

#### Scenario: Two Codex panes share a working directory

- **WHEN** a hook cannot be matched to a unique session binding and two live Codex panes have
  the same cwd
- **THEN** gtmux does not attach the event or receipt to either pane

### Requirement: HQ session replacement records preserve agent identity

New HQ session replacement records SHALL store the successor and predecessor agent keys with
their session ids in structured fields. Readers SHALL continue to interpret existing summary
only records using the existing conservative inference behavior.

#### Scenario: HQ changes from Claude to Codex

- **WHEN** a new Codex HQ session replaces a Claude HQ session
- **THEN** the history chain preserves both agent identities even if the predecessor log is
  later unavailable
