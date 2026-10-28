## ADDED Requirements

### Requirement: Browser appearance preference
The browser SHALL offer System, Light and Dark appearance modes, default to System, persist an explicit choice per browser origin, apply the resolved mode before first paint, and react to system appearance changes only while System is selected. The workspace, conversation, trace, controls and overlays SHALL adapt; terminal captures SHALL retain their recorded terminal palette.

#### Scenario: System appearance changes
- **WHEN** System is selected and the operating system changes to dark appearance
- **THEN** browser chrome updates without reloading or losing drafts

### Requirement: Closeable trace details
The trace inspector SHALL be closeable. Closing it SHALL give the event list the full available width and allow longer previews. Closing SHALL survive polling and filtering; selecting an event or choosing Show details SHALL reopen the inspector with its selection preserved.

#### Scenario: Read with the inspector closed
- **WHEN** the user closes event details and a transcript update arrives
- **THEN** the event list remains full width and the inspector remains closed

### Requirement: Recognizable browser identity
The browser SHALL provide a bundled gtmux favicon and a localized contextual tab title for the workspace, access page and focused conversation modes. Workspace branding and navigation SHALL provide readable size and spacing on desktop and narrow screens.

#### Scenario: Identify a conversation tab
- **WHEN** a user opens Trace for an agent
- **THEN** the tab title identifies that conversation, Trace and gtmux, and shows the bundled favicon
