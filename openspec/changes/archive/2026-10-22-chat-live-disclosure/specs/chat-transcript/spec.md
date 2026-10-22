## ADDED Requirements

### Requirement: Working-screen previews are opt-in
Phone and iPad conversations SHALL default to a compact working/thinking row with known elapsed time, an explicit Show live view / Hide live view disclosure and an accessible expanded state. The screen preview SHALL be bounded and scrollable and SHALL mount only while expanded. Ordinary screen/transcript refresh SHALL preserve the reader’s choice. A new working turn, pane or source session SHALL start collapsed. An unknown start SHALL not invent elapsed time. Pending input/choices and manual interaction SHALL remain independently visible and operable. Native/read-only conversations without a terminal screen SHALL retain the status without an empty preview disclosure.

#### Scenario: Inspect progress without persistent raw output
- **WHEN** a working conversation receives terminal output
- **THEN** only the compact status row appears by default
- **AND** tapping its disclosure reveals the screen and tapping again hides it
- **AND** an ordinary refresh preserves that choice

#### Scenario: A new turn needs attention
- **WHEN** a working turn ends or another turn/session starts
- **THEN** the raw working preview does not remain automatically expanded
- **AND** waiting choices continue through the separate input interaction
