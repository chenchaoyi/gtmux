# agent-screenshots Specification

## Purpose
Provide local agents with explicit, bounded screenshot selection, annotation and addressed delivery through the existing native screenshot engine and a versioned CLI contract.

## Requirements

### Requirement: Explicit screenshot authority and coordinates
The system SHALL require an explicit window or display source, short-lived owner/incarnation/geometry-bound reference and a source-local region. It SHALL revalidate the source before and after capture. Image and preview metadata SHALL include dimensions, exact hashes and coordinate references. Import SHALL retain imported provenance. Unknown or changed sources SHALL refuse without widening capture or requesting permissions implicitly.

#### Scenario: A window changes owner or geometry
- **WHEN** a source changes after discovery or during capture
- **THEN** no artifact is published and a new observation is required

### Requirement: Shared native renderer and private bounded transport
The CLI SHALL use the existing native renderer/editor through same-user descriptor-scoped Unix transport with verified host incarnation, nonce, operation allowlist and bounded deadline, JSON, image and point budgets. It SHALL NOT launch the App, run arbitrary commands, expose TCP, overwrite an existing editor draft or write caller-named native paths. Outputs SHALL publish without overwriting existing files after metadata and bytes agree. A late native writer SHALL NOT mutate published artifacts.

#### Scenario: Explicit annotation and editor roundtrip
- **WHEN** an exact image frame and valid structured marks are supplied
- **THEN** the existing renderer produces a derived image and preserves the base plus marks for explicit editing
- **AND** solid redaction removes covered pixels from the derived output while the original stays private and unchanged

### Requirement: Exact addressed delivery and truthful durable receipts
Screenshot delivery SHALL preserve existing draft, approval, uncertainty and no-replay guards. An on-demand target reference SHALL bind exact session, home, client, pane and server incarnation. A durable request claim SHALL precede mutation; changed payloads under the same ID SHALL refuse. Receipt queries SHALL describe the last observed acceptance state and SHALL NOT claim agent reading, completion or current queue membership. Automatic replay after uncertain acceptance SHALL NOT occur.

#### Scenario: A target changes or a response is lost
- **WHEN** the pane/session incarnation changes or acceptance is unconfirmed
- **THEN** no new recipient is silently selected and the request remains blocked or uncertain without automatic resend
