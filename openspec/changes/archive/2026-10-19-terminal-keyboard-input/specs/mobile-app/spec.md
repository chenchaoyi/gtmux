## ADDED Requirements

### Requirement: Discoverable terminal Keys panel

The phone and iPad SHALL provide a visible Keys entry beside the composer keyboard
toggle. The panel SHALL consume the shared catalog and offer Navigation, Function
and Characters categories, including advertised F1–F12 events. It SHALL retain the
existing frequent-control order, preserve the target's draft/attachments and adapt
to compact/regular layouts, large text and at least 44 × 44 pt touch targets.

#### Scenario: Send F2 without opening the software keyboard
- **WHEN** an input-authorized user opens Keys, selects Function and taps an available F2
- **THEN** one F2 event is sent without an appended Enter or submission of the saved composer draft

#### Scenario: Large text in compact layout
- **WHEN** the function-key grid no longer fits with accessible labels and touch targets
- **THEN** the panel reduces columns or scrolls its content while keeping Close reachable
- **AND** it does not shrink touch targets to fit

### Requirement: One-shot modifiers and honest key feedback

Ctrl/Alt/Shift selections SHALL modify only the next explicitly tapped target key,
and SHALL send nothing by themselves. They SHALL clear after that tap, dismissal,
target change, backgrounding or input-permission loss. Unsupported combinations
SHALL have accessible reasons. The initial panel SHALL NOT lock modifiers, auto-repeat
keys, execute macros, replay offline keys or capture global hardware shortcuts.

#### Scenario: Shift then Left
- **WHEN** a user selects Shift and taps Left
- **THEN** the selected combination is captured as one event and Shift is cleared
- **AND** further sends wait for its result rather than forming an invisible offline queue

#### Scenario: Panel dismissal
- **WHEN** a user selects modifiers and dismisses the panel without tapping a target
- **THEN** no input is sent, modifier selection is cleared and the draft remains intact

#### Scenario: Uncertain delivery
- **WHEN** a key send loses its response and delivery cannot be established
- **THEN** feedback says the result is unconfirmed rather than executed or safely unsent
- **AND** the client retains the exact event without automatically replaying it
