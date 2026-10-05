# mobile-app (delta)

## ADDED Requirements

### Requirement: The Servers page says which Macs answer and which one is open

The Servers page SHALL show two facts about every Mac, each with its own mark:

- **which one is open.** A check mark SHALL lead the open Mac's name, and only that one;
- **whether it answers.** Every row SHALL have a status line, whose dot and words SAY
  whether the Mac answers.

For the open Mac, the status line SHALL be the live connection's state:

- connected (filled green dot);
- connecting (amber);
- refused this phone (red, "Access rejected", as the refusal requirement says);
- cannot be reached (red).

For every other Mac, it SHALL be what a probe of that Mac found (hollow dot):

- answers (green);
- cannot be reached (red);
- checking (grey), before the first probe returns.

The probe SHALL be the unauthenticated `GET /api/health`, sent to each Mac while the page
is shown: on arrival, and every 15 seconds after, with a short timeout. A Mac that answers
the probe may still refuse this phone; the page SHALL NOT claim more than that it answers.

Every row SHALL be exactly two lines high whatever its state. A pending notification
setting SHALL be said on the status line, after the reachability ("cannot be reached ·
notification setting syncs when it answers"), never on a line of its own. A sync in
flight SHALL NOT be shown at all. So nothing the reader taps, and nothing the probes
find, resizes a row or moves the list.

#### Scenario: Three Macs, one open

- **WHEN** the phone is paired to Home (open, connected), Office (off) and a third Mac
  that answers
- **THEN** Home carries the check mark and "Connected" with a filled green dot
- **AND** Office reads "Can't reach" with a hollow red dot, and the third "Available" with a
  hollow green dot

#### Scenario: A setting that cannot reach its Mac

- **WHEN** Office cannot be reached and its notification setting is pending
- **THEN** Office's status line says both, on the one line, and offers no retry control
- **AND** when a later probe finds Office answering, the setting is sent again by itself

#### Scenario: Tapping does not move the list

- **WHEN** the reader taps a bell, or taps another Mac to switch to it
- **THEN** every row keeps its height while the switch or the sync is in flight

### Requirement: The radar's title switches Mac

The radar's title SHALL be the button that opens the Servers page, and SHALL look like
one:

- the connection dot SHALL lead the open Mac's name;
- a chevron in the brand colour SHALL follow it;
- the whole run (dot, name, chevron) SHALL be one target.

There SHALL be no separate switch glyph: a grey ⇄ square between the name and the dot
went unnoticed (2026-10-05). The dot keeps its connection colours, and its server-mode
ring. Accessibility SHALL name the action and the Mac ("Switch Mac: <name>").

#### Scenario: The reader looks for how to switch

- **WHEN** the radar is open on a Mac
- **THEN** its title reads "● <name> ⌄" with a brand-coloured chevron, and tapping any part
  of it opens the Servers page
