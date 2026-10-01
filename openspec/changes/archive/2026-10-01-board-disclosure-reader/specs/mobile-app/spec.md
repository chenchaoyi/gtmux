## ADDED Requirements

### Requirement: Board disclosures preserve a readable outline and source detail

The phone and iPad board reader SHALL use a shared 16-point stroked chevron for sections, entries and folded pane rows. Whole-row disclosure buttons SHALL have a touch target of at least 44 points, press feedback and an accessible expanded state. Long prose SHALL offer labelled Show full text / Show less actions in the app language.

Folded pane rows SHALL prioritize an explicitly labelled task when present and retain the original pane ID and location as secondary context. Expanded fields SHALL place their labels above selectable values and permit long values to reveal independently. Only known field labels SHALL be localized; unknown headings and source values SHALL remain intact. Generic nonfolded Markdown SHALL retain its existing presentation.

The outline SHALL preserve author order, omit empty entries and avoid repeating a decision body already lifted to the top. Child entries of a lifted section SHALL remain accessible. Inserting differently named sections, entries or pane rows during polling SHALL retain the user's expansion state for existing content.

#### Scenario: Read a pane with a long status

- **WHEN** a pane table has a task, location and a long status field
- **THEN** its folded row shows the task and pane/location context; opening it exposes labelled fields and a full-text action without changing the source

#### Scenario: Empty or promoted decision entry

- **WHEN** the recognized decision heading is empty or its body is displayed above the outline
- **THEN** no empty or duplicate body control appears, and any supporting child entries remain accessible

#### Scenario: Board polling inserts earlier content

- **WHEN** a poll inserts a differently named entry or pane before an open one
- **THEN** the existing entry or pane remains open, with its current source text

#### Scenario: Reachable bilingual controls

- **WHEN** a user taps a header or the full-text action in English or Chinese
- **THEN** the whole header responds, the action has a readable label, and assistive technology receives its expanded state
