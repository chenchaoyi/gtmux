## ADDED Requirements

### Requirement: Restore output separates the inventory from diagnostic detail

Normal `gtmux restore` output SHALL show a compact session inventory with window, pane and resumable-agent counts, without historical prompts. The existing read-only `--plan` preview SHALL retain bounded conversation excerpts, while `--plan --json` SHALL preserve its contract. Human rows SHALL use display-width-aware truncation. Layout warnings SHALL show the affected and saved totals, bounded short per-window facts and a full-log command; full geometry and diagnoses SHALL remain in the restore log. Progress SHALL NOT claim a faithful layout or recovered screen text before that is established.

#### Scenario: A large workspace has long historical prompts

- **WHEN** normal restore prints its inventory
- **THEN** each session occupies one compact row and historical prompts appear only in the explicit preview
- **AND** missing transcripts are counted separately from resumable agents

#### Scenario: Several restored windows differ

- **WHEN** pane counts, geometry or missing windows differ from the save
- **THEN** the warning retains both totals and short per-window facts without repeating the full explanation
- **AND** the complete diagnosis remains available through the printed restore-log command

## MODIFIED Requirements

### Requirement: A restore verifies the layout it restored

After a successful restore, the system SHALL compare each SAVED window's pane count and
arrangement against the window that came back, and SHALL report every window that
differs — to the restore log in full, and to the user as a bounded summary.

Neither side of the restore reports this today: the system confirmed only that session
NAMES came back, and the restore plugin applies window properties with its errors
discarded, so a window whose pane count no longer matches the save is refused a layout by
the multiplexer ("have 3 panes but need 2") and silently keeps a default arrangement.
Two windows failed this way (2026-08-15, 2026-08-18) and neither produced a single line
of evidence.

The comparison SHALL be made on a NORMALIZED layout — the checksum and the per-pane
numbers removed — because a restarted server reissues pane ids, so a raw comparison
reports every window as changed. Pane-count drift SHALL be reported in preference to
arrangement drift for the same window, since a pane-count mismatch is the CAUSE of most
arrangement drift rather than another symptom. A saved window with no live counterpart
SHALL be reported; a LIVE window the save never had SHALL NOT be (sessions created after
the save are normal, and reporting them would bury the signal).

#### Scenario: A window comes back with the wrong number of panes

- **WHEN** a window recorded with 2 panes comes back with 3
- **THEN** the restore names that window and both pane counts, on the terminal and in the
  restore log; the terminal uses a short count-change fact and the log explains why
  the layout could not be applied

#### Scenario: A faithful restore says nothing

- **WHEN** every saved window comes back with the same pane count and arrangement — pane
  ids renumbered by the new server notwithstanding
- **THEN** no drift is reported

#### Scenario: A session created after the save is not drift

- **WHEN** the running server has a session the save never recorded
- **THEN** it is not reported as drift
