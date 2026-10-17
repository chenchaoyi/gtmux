## ADDED Requirements

### Requirement: Conversation entry reveals current content without disrupting reading
Opening or reselecting Talk to HQ or a matter SHALL reveal the newest loaded conversation, including cached content. Refresh while deliberately reading older messages SHALL preserve the reading position; a visible new-message action SHALL return to the latest content. Earlier-page loading and drafts SHALL retain their existing scope and content. Composer destination labels SHALL name HQ, the new task or the actual matter without internal routing prose or implying that matter association is impossible.

#### Scenario: A cached conversation is opened again
- **WHEN** the selected conversation already has loaded history before its view appears
- **THEN** the view opens at its newest loaded content
- **WHEN** the user scrolls into older history and refresh adds a reply
- **THEN** the reading position is retained
- **WHEN** the user clicks the same conversation destination again
- **THEN** the latest loaded content is revealed without discarding the draft or history
