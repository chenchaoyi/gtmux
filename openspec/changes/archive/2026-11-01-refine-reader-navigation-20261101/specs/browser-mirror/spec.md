## ADDED Requirements

### Requirement: Distinct adaptive trace detail navigation
Trace SHALL distinguish returning from record details from closing the tool. A compact
reader SHALL provide a leading Back action and a wide inspector a trailing Hide details
action. Resizing between phone, tablet and desktop widths SHALL retain the selected
original call, contain the inspector and preserve keyboard focus when details close.

#### Scenario: Resize an inspected call through tablet multitasking widths
- **WHEN** the viewport changes through portrait, landscape and narrow tablet widths
- **THEN** the original selected source remains readable without widening the page
- **AND** closing details restores focus to its original record without sending input
