## ADDED Requirements

### Requirement: Independent trace disclosures and detail modes
Browser Trace SHALL provide independent Turns and Calls bulk disclosures. Every
contiguous call run, including one or two calls, SHALL remain independently openable
as a counted row with tool names. Explicit bulk defaults SHALL apply to new groups;
individual choices SHALL survive same-scope refresh. Filtering SHALL reveal matches
without mutating those choices. Folding-induced scrolling SHALL NOT resume tail
following. The rendered page bound SHALL include projected group rows.

#### Scenario: Read a collapsed run while new activity arrives
- **WHEN** calls are collapsed and one run is opened before more records arrive
- **THEN** the opened run and visible reading anchor remain stable and new runs inherit the bulk choice
- **AND** expanding Calls does not expand collapsed Turns

#### Scenario: Inspect context and return from details
- **WHEN** an authorized reader selects recorded context
- **THEN** Overview, Preview, Raw content and Source show actual facts, formatted text, original text and provenance respectively
- **AND** no absent model request, call timestamp, timing, token count or tool result is invented
- **AND** closing a record hidden by folding returns focus to its visible parent disclosure

### Requirement: Navigable compact browser settings
Browser Settings SHALL use Appearance, Chat and Terminal sections with short hints,
category navigation and bounded internal scrolling. Wide panels SHALL use side
navigation and narrow panels a horizontal row. Light, Dark and System SHALL have
recognizable icons and native radio behavior. Existing immediate/persisted preference
semantics, Escape/focus return and desktop/touch target sizes SHALL remain effective.

#### Scenario: Change system appearance from a narrow panel
- **WHEN** a reader opens Settings at 320px and selects System
- **THEN** all categories and choices remain reachable without page overflow
- **AND** system changes update appearance while explicit Light or Dark overrides them
