## ADDED Requirements

### Requirement: Structured transcript activity

Phone, iPad and Web SHALL display each transcript tool-step segment as a neutral disclosure card with an activity heading, total call count and compact tool-count summary. Expanded cards SHALL preserve call order and show the recorded tool name separately from its bounded argument summary. Long summaries and names SHALL wrap without hiding adjacent conversation content.

Codex summaries SHALL prefer non-empty string `title`, then `description`, before existing command/path fallbacks, retaining the existing 80-rune limit. Surfaces SHALL render summaries as plain text and SHALL NOT infer per-call success, completion, duration or output from the existence of a call. The phone/iPad running turn SHALL keep its existing open-by-default policy, completed history SHALL stay folded, and explicit reader choices SHALL survive updates and dropped older turns.

#### Scenario: Repeated browser calls

- **WHEN** a segment records six `js` calls with descriptions and one `wait`
- **THEN** its card counts seven calls, summarizes the repeated tools, and shows descriptions beside the actual tool names when expanded
- **AND** no call is labelled successful from its presence in the log

#### Scenario: An older core provides only tool names

- **WHEN** steps have no argument summaries
- **THEN** their ordered tool names remain readable and the card can still be folded
