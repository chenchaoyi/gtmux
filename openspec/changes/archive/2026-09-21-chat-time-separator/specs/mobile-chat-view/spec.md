# mobile-chat-view (delta)

## REMOVED Requirements

### Requirement: Per-turn time label with date

**Reason**: The label carried HH:MM and was shown whenever it differed from the turn
above, so it appeared above almost every turn and marked nothing.
**Migration**: The same label, on the same timestamp and in the same wording, is now shown
at a break rather than at a minute, and drawn as a separator.

## ADDED Requirements

### Requirement: The chat marks where the conversation broke

The chat SHALL mark where the conversation BROKE, not where the clock changed.

A separator SHALL appear above a turn when it is the first turn carrying a timestamp,
when its calendar day differs from the turn before it, or when it follows the turn before
it by at least a configured gap. A turn meeting none of those SHALL carry no time of its
own, so that a separator means a pause rather than a minute.

The label SHALL be derived from the prompt timestamp and include the date for clarity
(relative "today/yesterday" wording, calendar date otherwise, the year when not the
current year), localized en/zh.

It SHALL be drawn as a separator and not as a line of text: the label centred, with a rule
either side of it, so a reader scrolling back sees the breaks rather than reading clocks.

A turn with no usable timestamp SHALL never produce a separator, and SHALL NOT break the
comparison for the turns around it.

#### Scenario: A working back-and-forth

- **WHEN** turns arrive a few minutes apart through one sitting
- **THEN** no separator appears between them

#### Scenario: Coming back after a pause

- **WHEN** a turn follows the one before it by at least the gap
- **THEN** a separator above it gives the date and time, centred between two rules

#### Scenario: A new day

- **WHEN** a turn's calendar day differs from the turn before it
- **THEN** a separator appears, whatever the gap

#### Scenario: The start of the history

- **WHEN** the first turn carrying a timestamp is shown
- **THEN** it carries a separator, so the conversation has a beginning
