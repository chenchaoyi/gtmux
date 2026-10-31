## ADDED Requirements

### Requirement: Adaptive reader hierarchy and dismissal
Phone and iPad SHALL share reader components. Trace details SHALL use Back on compact
canvases and Hide details on wide canvases, distinct from the single outer Close.
Resizing or large text SHALL retain the selected source. Native dismissal SHALL retain
the themed painted tree until completion without freezing retired authorization data.
Trace, Diff, code readers, profiles and menus SHALL give Escape only to the foremost
active overlay. Large-text Diff SHALL keep gutters and wrapped content in its viewport.

#### Scenario: Leave a selected call on iPad
- **WHEN** a selected Trace call moves from a wide canvas to a narrow multitasking window
- **THEN** the same source stays selected and the detail action becomes Back to trace
- **AND** Escape leaves details before Trace without acting on the page underneath

#### Scenario: Dismiss a dark Trace during a scope change
- **WHEN** the native dismissal starts while the identity or authorization retires data
- **THEN** the theme stays painted through dismissal and old private props are not frozen
