## ADDED Requirements

### Requirement: Independent native trace disclosures and provenance
Phone and iPad Trace SHALL share independent Turns/Calls bulk disclosures and
individually openable counted call runs, including runs of one or two calls. Bulk
choices SHALL apply to new records, individual choices SHALL survive refresh, and
filtering SHALL reveal matches temporarily. A folding-induced scroll SHALL NOT resume
following. Context SHALL retain recorded order without generating human messages.
Overview, Preview, Raw content and Source SHALL share the frozen selected revision,
use actual provenance and retain original text for copy outside Source.

#### Scenario: Fold turns independently from calls during streaming
- **WHEN** the reader collapses turns then expands calls while new records arrive
- **THEN** turns remain collapsed and new groups inherit the corresponding bulk defaults
- **AND** a programmatic tail scroll does not move the reader to new activity

#### Scenario: Inspect a recorded context revision
- **WHEN** the reader switches detail tabs and a selected context record changes
- **THEN** the displayed original revision stays frozen until explicitly updated
- **AND** Source exposes recorded origin and available clocks without inventing a model request
