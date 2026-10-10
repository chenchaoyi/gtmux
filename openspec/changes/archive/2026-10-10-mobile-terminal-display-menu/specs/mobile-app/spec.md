## MODIFIED Requirements

### Requirement: Original terminal layout preserves captured rows

The phone and iPad Detail terminal SHALL default to Fit screen (wrapped rows). A neutral display icon in the terminal toolbar
SHALL open the shared anchored menu offering Fit screen, Preserve layout (original row widths), and text-size adjustments.
The chosen layout SHALL show a checkmark and selected accessibility state; menu rows SHALL be at least 44 pt high.
The terminal SHALL NOT overlay a bottom-left layout control or reserve padding for one. Chat SHALL keep its existing text-size
controls. Full-screen SHALL retain the selected layout; users SHALL exit full-screen to adjust it.
Original SHALL preserve captured physical row boundaries, indentation, ANSI colours and cursor/selection geometry,
using a canvas at least as wide as the viewport, reported pane columns, widest captured row in display cells and cursor.
When pane columns are absent, the captured rows SHALL determine the width. History wider than the current pane SHALL
remain readable by horizontal scrolling. Switching layouts SHALL NOT resize the Mac pane, invent paragraph joins or
recover bytes Codex has truncated; the separate pinned-prompt reader remains responsible for that content.
Both layouts SHALL retain the same vertical scroller and selection layer. Polling SHALL preserve the selected mode.
On iOS, switching away from the live tail SHALL preserve the visible captured line where it remains in the snapshot.
Active native text selection SHALL disable layout and text-size changes in the display menu. Android SHALL preserve source rows in Original with its
existing native text-selection overlay and horizontal scrolling.

#### Scenario: Codex history formatted for a wide Mac pane

- **WHEN** a 189-column capture contains mixed Chinese and Latin text with indented continuation rows
- **THEN** Wrap may fit each captured row to the phone width, while Original retains exactly the captured row boundaries
- **AND** the user can scroll horizontally without remounting the vertical scroller

#### Scenario: Older server or narrower Mac pane

- **WHEN** the capture includes a row wider than the reported pane, or the server supplies no columns
- **THEN** Original measures display cells in the captured rows rather than ANSI bytes and keeps the widest row readable

#### Scenario: Refresh while reading or selecting

- **WHEN** Original is selected and a new snapshot arrives
- **THEN** Original remains selected and unchanged rows retain their render identities
- **AND** changing layouts during a native text selection is disabled
