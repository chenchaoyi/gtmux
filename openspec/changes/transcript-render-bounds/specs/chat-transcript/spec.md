# chat-transcript — delta

## ADDED Requirements

### Requirement: The served transcript is bounded by size, not only by turn count

The system SHALL bound the transcript payload it serves by its SIZE, keeping the most
recent turns that fit a byte budget and dropping older ones. A bound on the NUMBER of
turns is not sufficient and SHALL NOT be relied on alone: turns vary in size by orders of
magnitude, so a turn count cannot imply a payload a client can hold — a session well
under any reasonable turn cap has produced a payload large enough to exhaust a phone's
memory on parse and render. The system SHALL report how many turns were dropped, so a
client can tell the user the history is truncated instead of presenting a partial
conversation as if it were complete. It SHALL always serve at least the most recent turn,
even when that single turn exceeds the budget, since serving nothing is worse than
serving the part the user is looking at.

#### Scenario: A long conversation is truncated to its tail

- **WHEN** a session's parsed history exceeds the payload budget
- **THEN** the most recent turns that fit are served, the older ones are dropped, and the
  response reports how many were dropped

#### Scenario: A short conversation is served whole

- **WHEN** a session's parsed history fits within the budget
- **THEN** every turn is served and nothing is reported as dropped

#### Scenario: One oversized turn is still served

- **WHEN** the single most recent turn is by itself larger than the budget
- **THEN** it is still served, rather than returning an empty history
