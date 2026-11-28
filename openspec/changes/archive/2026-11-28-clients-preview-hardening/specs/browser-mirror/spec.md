## ADDED Requirements

### Requirement: Visibility-gated browser polling and conditional transcript reads
The browser mirror SHALL stop Workbench tile terminal polling when the Workbench is inactive or the tile is removed, pause active radar, pane, chat, and options polling while `document.hidden` is true, trigger an immediate refresh when `visibilitychange` restores visibility, and send per-pane/per-tile `If-None-Match` headers on chat transcript polls so `304 Not Modified` responses skip JSON parsing and DOM updates.

#### Scenario: Hidden browser tab and unchanged chat transcript
- **WHEN** the browser tab is hidden or a chat poll receives `304 Not Modified` for its current `ETag`
- **THEN** hidden pollers issue no requests until `visibilitychange` refreshes the active view, and `304` responses short-circuit without parsing JSON or rebuilding the chat DOM
