## ADDED Requirements

### Requirement: Consistent browser controls
The browser SHALL use a shared icon system for action controls, with 20px action glyphs, at least 36×36 CSS-pixel desktop targets and 44×44 coarse-pointer targets. Compact tiles SHALL preserve the same target metrics. Action controls SHALL have localized accessible names, visible focus and hover feedback. Layout and appearance disclosures SHALL expose expanded state, support Escape dismissal and return focus. Session rows and authorized reply choices SHALL support keyboard activation.

#### Scenario: Use compact controls by keyboard or touch
- **WHEN** the reader uses a tile's navigation, message actions or composer keys
- **THEN** controls remain legible, touch targets retain their size and authorized actions can be reached by keyboard

### Requirement: Message footer and truthful clipboard feedback
Conversation actions SHALL appear outside the message background in a separate footer. Recorded prompt timestamps and sender attribution SHALL appear in that footer; absent or invalid times SHALL be omitted. Clipboard actions for messages, pinned prompts, pane commands and terminal captures SHALL indicate the actual copy result, including failure of the fallback copy path.

#### Scenario: Clipboard access fails
- **WHEN** both the clipboard API and fallback copy fail
- **THEN** the interface reports failure rather than success and preserves keyboard focus

### Requirement: Consistent browser interface language
Browser routes and overlays SHALL use concise localized action labels, factual empty states and explicit recovery messages. Chinese copy SHALL distinguish the workspace, panels, agent conversations and tmux panes consistently. Agent transcripts and literal commands SHALL remain unchanged. Appearance settings SHALL state that font controls affect terminal rendering.

#### Scenario: Read an empty layout menu
- **WHEN** no layout has been saved
- **THEN** the menu states that no saved layouts exist and offers a clearly named Save current layout action
