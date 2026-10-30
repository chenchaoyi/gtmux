## ADDED Requirements

### Requirement: Stable conversation text layout during refresh
The iOS conversation SHALL initially constrain prose to its available container width rather than a one-pixel placeholder. It SHALL retain the last accepted height during content refresh and reuse a short natural width only for unchanged text layout. Changed prose SHALL regain the available width. It SHALL ignore stale measurements, avoid publishing constrained sizes before a usable width exists, and avoid rebuilding identical native documents. Unbounded code sizing and native selection SHALL remain available.

#### Scenario: Polling and delayed measurements
- **WHEN** a visible conversation refreshes while native size events are delayed
- **THEN** existing prose retains its accepted height and native view identity
- **AND** newly mounted prose starts at the available container width rather than briefly displaying only its first character

#### Scenario: Unresolved and changed container width
- **WHEN** constrained text has no usable layout width, or its container is resized
- **THEN** an unresolved width cannot publish a collapsed single-line height
- **AND** a usable new width remeasures the full text while unbounded code can still report its natural width

#### Scenario: A short message grows
- **WHEN** a one-character message grows into a full paragraph
- **THEN** it regains the available constrained width immediately while retaining its last height until remeasurement
- **AND** a source/link revision with unchanged text layout can retain the measured natural width
