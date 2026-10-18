## ADDED Requirements

### Requirement: Conversation block copying has visible feedback

On iOS, long-press Copy in a conversation SHALL visibly highlight the entire text
block that the command copies while the menu is open. Markdown formatting, inline
links, font scaling and existing block spacing SHALL remain available. This SHALL
NOT imply range selection or selection across blocks. Non-conversation Markdown
and Android selection SHALL retain their existing behavior.

#### Scenario: Copy a formatted paragraph

- **WHEN** the user long-presses a formatted conversation paragraph and chooses Copy
- **THEN** that paragraph is highlighted while the menu is open and its rendered
  text, including line breaks, is copied without unrelated response blocks

#### Scenario: The menu becomes stale

- **WHEN** a menu is dismissed, its block is removed or its text changes
- **THEN** the highlight is removed and the old menu cannot copy new text; a late
  dismissal callback cannot clear a newer selection

#### Scenario: The user navigates with accessibility actions

- **WHEN** a conversation text block exposes its Copy accessibility action
- **THEN** the action has a localized label and copies the same block as long-press

## MODIFIED Requirements

### Requirement: Interactive controls use spoken labels

The phone and iPad SHALL give interactive controls a short, meaningful label in the
selected language. Automation identifiers SHALL live in `testID` and SHALL NOT be
spoken as accessibility labels. Connection-page helper text SHALL remain legible in
light and dark appearance.

#### Scenario: A screen reader reaches the composer

- **WHEN** VoiceOver focuses send, attachment, or expand
- **THEN** it hears the action in the selected language rather than a test identifier

#### Scenario: The user reaches HQ or a numbered answer

- **WHEN** accessibility focuses the HQ card or an approval choice
- **THEN** HQ names its opening action and fleet state, and each choice names its
  number and actual answer, with a button role; automation retains its test identifier
