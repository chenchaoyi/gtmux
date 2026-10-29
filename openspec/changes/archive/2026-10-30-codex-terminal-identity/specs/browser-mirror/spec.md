## MODIFIED Requirements

### Requirement: Closeable trace details

The trace inspector SHALL be closeable. Closing it SHALL give the event list the full
available width and allow longer previews. Closing SHALL survive polling and filtering;
selecting an event or choosing Show details SHALL reopen it. Width-appropriate compact
views SHALL drill into details and return to the same selected row and focus, rather
than dividing an already narrow reading area. Identifiable records SHALL retain their
selection across history additions. Selected text and focused detail controls SHALL
hold their displayed revision during updates to the SAME event and resume after release. If the selected event disappears when a session is replaced, the inspector SHALL immediately show the newly selected event rather than retain the old session under the new list. Inspecting or
manually scrolling SHALL stop automatic following. Viewer-owned observers and timers
SHALL be disposed when the trace is retired.

#### Scenario: Read with the inspector closed

- **WHEN** the user closes event details and a transcript update arrives
- **THEN** the event list remains full width and the inspector remains closed

#### Scenario: Read a call on a phone or compact tile

- **WHEN** a reader selects a record in a narrow content pane
- **THEN** details use the reading area and the underlying list is not an interactive accessibility target
- **AND** closing details restores the list and focuses the original row

#### Scenario: Preserve reading while history changes

- **WHEN** older or newer identifiable history is inserted while a record is selected
- **THEN** the same record remains selected, without substituting another record at its previous array index
- **AND** a selected text range remains intact until the reader releases it

#### Scenario: Session changes while an old call is selected
- **WHEN** the current transcript replaces the selected event and its old source text remains selected or focused
- **THEN** the inspector immediately displays the newly selected event with matching session metadata on desktop and compact layouts
- **AND** ordinary streaming updates to an existing selected event still preserve the reader's selection
