# mobile-app (delta)

## ADDED Requirements

### Requirement: The app is universal and one size-class rule decides its shell

The app SHALL build for iPhone and iPad from one target, and SHALL choose between two
shells from one rule: `regular` when the window is at least 768 points wide and 600
points tall (a sidebar with the radar beside a main pane), `compact` otherwise (the
phone's stack, unchanged). Screens SHALL NOT read the window size to choose a layout
themselves.

#### Scenario: iPad in landscape or portrait

- **WHEN** the app runs on an iPad in any orientation, or in a Stage Manager window at
  least 768×600
- **THEN** the radar is a sidebar and the selected pane, the HQ page or All panes
  fills the main pane

#### Scenario: iPad Split View at one half, or Slide Over

- **WHEN** the window is narrower than 768 points or shorter than 600
- **THEN** the app shows the phone's stack, and a rotation or a resize that crosses the
  line switches shells without losing the selection

### Requirement: The regular shell composes the phone's screens, never copies them

The sidebar SHALL render the same radar component the phone screen renders, differing
only by props (how a row is opened, which row is selected, its width, which HQ entry it
carries). Detail, the HQ page and All panes SHALL be the same components on both shells.
A structural test SHALL fail the build when a second radar chrome, a screen picking its
own layout from the window size, or the old split screen reappears.

#### Scenario: A radar feature lands on the phone

- **WHEN** a section, banner or row affordance is added to the radar
- **THEN** the iPad sidebar shows it in the same commit, with no second implementation

### Requirement: What is open is workspace state

The selected pane, the HQ page or All panes SHALL be held in one workspace context. The
compact shell SHALL map a selection to navigation and the regular shell to the main
pane; push deep links, keyboard commands and in-app "open" actions SHALL set the
selection and never address a shell.

#### Scenario: A push opens a pane on the iPad

- **WHEN** a waiting push is tapped while the regular shell is showing
- **THEN** that pane is selected in the sidebar and shown in the main pane, with no
  screen pushed

### Requirement: Wide layouts where the width changes the reading

On the regular shell the HQ page SHALL show its report header across the main pane, the
console beneath it, and a right-hand inspector carrying "Your call" and "HQ's work"; All
panes SHALL lay session cards in a grid; the Knowledge sheet SHALL show its list beside
the open entry. Chat and the HQ console SHALL cap at a reading width and centre; the
terminal SHALL use the full width.

#### Scenario: A blocked session while the HQ page is open on an iPad

- **WHEN** a session starts waiting
- **THEN** its decision card appears in the inspector beside the console, without a tab
  switch

### Requirement: Hardware keyboard and pointer

On iPad the app SHALL register key commands from one keymap table: ↑/↓ move the radar
selection, ⏎ opens it, ⌘1–9 jump to a row, ⌘⇧H opens HQ, ⌘⇧P All panes, ⌘F the pane
search, ⌘K focuses the composer, esc closes a sheet, ⌘[ / ⌘] switch chat and terminal,
⌘+ / ⌘− change the font size, ⌃⌘S hides the sidebar. Rows and buttons SHALL show a hover
tint under a pointer. The ⌘-hold overlay SHALL list the commands with their titles.

#### Scenario: A keyboard user opens the third row

- **WHEN** ⌘3 is pressed with the regular shell showing
- **THEN** the third radar row is selected and its detail fills the main pane

### Requirement: The iPad ships with its own store screenshots

The App Store listing SHALL carry a 13" iPad screenshot set drawn by the same demo-mode
pipeline as the phone set, in both locales, framed for the iPad slot.

#### Scenario: A release stamps a new version

- **WHEN** `set-version.sh` runs and the app's source changed
- **THEN** the iPad set is regenerated with the phone set, and the design gate fails if
  either is missing
