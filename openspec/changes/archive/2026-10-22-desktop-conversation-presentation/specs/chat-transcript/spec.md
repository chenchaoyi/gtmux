## ADDED Requirements

### Requirement: Native external readers align with conversation interaction semantics

The native external reader SHALL group assistant segments by turn with one participant header, preserve core prompt attribution, render public Markdown and display compact source information. It SHALL offer local reply and global folding, icon-only copy/share with accessible names, and structured tool groups with operation icons and a scrollable recorded-input inspector. Copy/share SHALL include the complete recorded textual reply even when folded. Missing or truncated tool evidence SHALL be explicit and SHALL NOT imply execution success. Native colours SHALL follow light/dark appearance. Initial/latest navigation SHALL be bounded and ordinary refresh SHALL preserve deliberate historical reading. The implementation SHALL reuse existing native reading/avatar/copy/positioning primitives and keep the read-only CLI contract, polling and scope rules.

#### Scenario: A prompt was sent by HQ

- **WHEN** the core attributes a prompt to HQ or another agent
- **THEN** its native reader shows that sender and does not label it as the human reader

#### Scenario: A long tool input is inspected

- **WHEN** the user expands a recorded command or delegation call
- **THEN** the original recorded whitespace, working directory and truncation notice are available, with scrolling and copying
- **AND** collapsing the reply retains a preview and full textual copy/share actions
