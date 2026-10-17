## MODIFIED Requirements

### Requirement: New session prompt is compact and ready for typing

The New session action SHALL open a small, left-aligned window with one session-name
field. The field SHALL receive focus and show a selected concrete suggested name
(`work-` plus eight random hexadecimal characters), ready to accept or replace.
The same name SHALL remain as the placeholder after clearing the field. The label
and field SHALL share one compact row. The window SHALL size to native controls
and keep Cancel/Create immediately below. Opening or cancelling SHALL create nothing.
Create SHALL pass either the trimmed entered name or the exact displayed suggestion
as one argument to `gtmux new`. CLI and mobile blank-name defaults SHALL remain unchanged.

#### Scenario: Create a session without entering a name

- **WHEN** the user immediately confirms or clears the suggested name and confirms
- **THEN** the app passes the exact displayed suggestion as the name to `gtmux new`

#### Scenario: Type and create a named session

- **WHEN** the user replaces the selected suggestion and confirms
- **THEN** the app passes the trimmed entered name as one argument to `gtmux new`

### Requirement: Upgrade notes are readable once and revisitable
The menu-bar app SHALL show a compact, resizable What's new window after an
upgrade when the user first opens the menu, following the phone's newest-expanded/older-folded version grouping.
Hidden background launches SHALL NOT activate the reading window; a response after
the menu closes SHALL wait for the next opening.
It SHALL show only notes up to the installed app version and provide
a permanent Software update preferences entry. Prose SHALL wrap to the actual
window width and scroll; Return, Escape and window close SHALL dismiss the reader.
It SHALL acknowledge successfully read notes on dismissal, not on fetch start.
First installation SHALL be quiet; downgrade SHALL not lower acknowledgement.
Missing acknowledgement on an existing install SHALL migrate the prior launch
version before the environment check replaces that marker.

#### Scenario: Skip several releases
- **WHEN** the installed app is newer than the acknowledged version
- **THEN** the automatic reader includes each unseen version in the latest-30-release
  source range, newest first, without including releases newer than the installed app
- **AND** a source-range gap is disclosed with a release-archive entry

#### Scenario: Fresh install or acknowledged version
- **WHEN** there is no prior launch/acknowledgement, or the current version is at or below it
- **THEN** no automatic reader is shown; the manual entry remains available

#### Scenario: Read fails or is superseded
- **WHEN** the CLI times out, fails or returns invalid version/schema data
- **THEN** acknowledgement is unchanged and manual reading provides Retry
- **AND** a superseded automatic reply cannot replace a manual history request

#### Scenario: Reopen notes offline
- **WHEN** a valid bilingual receipt for the installed version is cached
- **THEN** the manual reader opens without a network request and changes language
  using the selected block, falling back to the other language when absent

## ADDED Requirements

### Requirement: Screenshot delivery owns the editor until its receipt
While sending, a centered gtmux delivery panel SHALL cover the blurred, inactive
editor and identify the captured destination and terminal-confirmation state.
Notes, annotations, target selection, export controls and keyboard/undo routes
SHALL all be unavailable; close SHALL be disabled while sending. Copy SHALL NOT
replace delivery state. Confirmed sent/queued results SHALL remain locked until
the existing automatic close, which SHALL target only that editor. Every refusal,
failure or unconfirmed receipt SHALL restore editing with the note and marks
preserved, without automatic retry. The panel SHALL NOT claim task completion.
Progress SHALL respect reduced-motion and reduced-transparency preferences.

#### Scenario: Export and undo during a send
- **WHEN** the user invokes Copy, Save, Undo or Send again while delivery is pending
- **THEN** no export, edit, extra delivery or status overwrite occurs
- **AND** the original undo history remains usable after a failed receipt

#### Scenario: A late success belongs to an older editor
- **WHEN** a successful editor schedules its close and another editor opens
- **THEN** the old close cannot dismiss the newer editor

### Requirement: Gear menu exposes frequent actions
The gear menu SHALL contain Preferences, Pair a device, Check for updates and Quit.
It SHALL NOT duplicate What's new or Environment check entries. Upgrade notes SHALL
remain automatically presented and manually accessible in Software update preferences;
post-update environment checks SHALL retain their existing flow.

#### Scenario: Open the gear menu
- **WHEN** a user opens the menu
- **THEN** the two secondary entries are absent while the four frequent actions remain
