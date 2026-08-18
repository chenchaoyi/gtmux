# session-restore (delta)

## MODIFIED Requirements

### Requirement: serve backstops the resurrect save

`gtmux serve` SHALL, on its slow tick, trigger a tmux-resurrect save ITSELF whenever a
tmux server is up AND the last resurrect save FILE has gone stale, so that save freshness
does not depend on the periodic autosaver being correctly armed OR on it actually
running. The staleness of the file — never the presence of the autosave trigger in the
multiplexer's status line — SHALL be the criterion. An autosave trigger that hangs off
the status line only fires while a client is attached and redrawing, so a sleeping
machine carries a perfectly-configured trigger and saves nothing; a gate on the trigger's
PRESENCE therefore reads as "handled" during exactly the outages it exists to cover
(measured: 76 gaps in 3.5 days, the longest just under six hours, with the backstop never
firing once).

Where the trigger IS present the system SHALL allow a WIDER (but finite) staleness grace
before stepping in, so an armed autosaver gets the first move. When the last save is
fresh it SHALL do nothing. The save SHALL run as a direct subprocess with `$TMUX` and a
robust PATH (NEVER `tmux run-shell`, which runs in the server's minimal-PATH environment,
exits non-zero, and can poison the `last` pointer with an empty save), and the `last`
pointer SHALL be repaired afterward if a save wrote an empty file. Concurrent backstop
saves SHALL be prevented (single-flight).

The health check that reports autosave status SHALL likewise report the save's observed
AGE, and SHALL flag an armed trigger that has not written the save for a long while
rather than reporting it healthy.

#### Scenario: Backstop fires when an armed autosaver is not actually saving

- **WHEN** the autosave trigger is present in the status line but nothing has written the
  save for longer than the armed grace (the machine slept, or no client was attached)
- **THEN** serve triggers a resurrect save itself, because written into the configuration
  is not the same as running

#### Scenario: Backstop fires when continuum is dead

- **WHEN** no autosave trigger is present and the save has gone stale past the shorter
  interval
- **THEN** serve triggers a resurrect save itself

#### Scenario: Backstop is a no-op when continuum is healthy

- **WHEN** serve's slow tick runs and the last resurrect save was written within the
  applicable interval
- **THEN** serve does not trigger a save — a saver that is working keeps the file fresh,
  and that freshness is itself the interlock against a second concurrent save

#### Scenario: Health check flags an armed trigger that has stopped saving

- **WHEN** the autosave trigger is present and the save has not been written for hours
- **THEN** the autosave health row is flagged with the observed age, not reported OK

### Requirement: The layout backstop never runs alongside the autosaver

The system SHALL NOT run its own layout save CONCURRENTLY with the periodic autosaver:
two save runs over the same files have produced duplicate save files and a truncated
pane-contents archive — the system corrupting the very save restore depends on — and
corrupting the save is strictly worse than the staleness the backstop guards against.

The interlock SHALL be the save's own FRESHNESS, not the autosaver's configuration. An
autosaver that is running keeps the save fresh, so a staleness-triggered backstop never
wakes beside it; an autosaver that is merely ARMED has been measured saving nothing for
hours, so treating "armed" as "running" disables the backstop during exactly the outages
it exists for. Where a trigger is present the system SHALL additionally (a) allow a wider
staleness grace before stepping in and (b) YIELD briefly and re-check the save before
running it, standing down if the autosaver saved in the meantime — the one moment both
plausibly fire together is a wake from sleep, when the status line redraws and the
service's tick resumes at the same instant.

The system SHALL also invoke the save script in its QUIET mode, because its default mode
paints a progress message into the multiplexer's message line on every attached client
and forks an extra process to animate it, producing recurring on-screen noise with no
visible cause.

#### Scenario: The autosaver is armed

- **WHEN** the periodic save trigger is present and saves are still landing
- **THEN** the system does not save the layout itself

#### Scenario: The autosaver is missing

- **WHEN** the periodic save trigger is absent and the save has gone stale
- **THEN** the system saves the layout itself, without printing to the message line

#### Scenario: The autosaver saves during the grace window

- **WHEN** the system has decided to step in for an armed autosaver and that autosaver
  writes the save during the yield
- **THEN** the system stands down instead of running a second, concurrent save

## ADDED Requirements

### Requirement: A restore names the moment it is restoring

`gtmux restore` SHALL, on every restore, state WHICH saved moment is coming back — the
save's clock time and its age — to the user and to the restore log.

The existing day-scale staleness warning is not sufficient for this, because the loss
that costs work is invisible at that resolution: a save 37 minutes old passes every
threshold and still omits everything done in those 37 minutes. Naming the moment turns
"restore broke my layout" into "the layout I am restoring is the one from 09:57".

#### Scenario: A recent save still says how recent

- **WHEN** restore resolves a save written 37 minutes ago
- **THEN** it prints the save's time and age, even though no staleness warning applies

#### Scenario: No save, no note

- **WHEN** no resurrect save can be resolved
- **THEN** no age line is printed

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
  restore log, explaining that the layout could not be applied

#### Scenario: A faithful restore says nothing

- **WHEN** every saved window comes back with the same pane count and arrangement — pane
  ids renumbered by the new server notwithstanding
- **THEN** no drift is reported

#### Scenario: A session created after the save is not drift

- **WHEN** the running server has a session the save never recorded
- **THEN** it is not reported as drift

### Requirement: Pane-keyed state is reconciled with the panes that exist

The system SHALL drop pane-keyed state records whose panes the multiplexer no longer has
— on completion of a restore, and periodically while `gtmux serve` runs.

A pane id is a per-server SEQUENCE NUMBER, not an identity: restarting the server
reissues it from the start, so a record that outlives its pane does not merely go stale,
it begins describing whichever pane inherited its number. Measured after one reboot:
50/52 enrollment records, 27/29 goal records, 31/32 send records and 93/103 wake records
named panes that had not existed for up to two weeks, while live panes carried those
numbers — producing one session's instructions attributed to another's pane, and a
two-week-old undelivered dispatch record steering a screen-scrape of an unrelated
project's pane into a fabricated alarm.

The sweep SHALL cover every pane-keyed family, SHALL delete only files whose NAME is a
pane id, and SHALL refuse to run on an EMPTY live-pane set (an unreadable pane query must
read as "unknown", never as "no panes exist"). Records keyed by LOCATOR or by
conversation id — the resume records restore itself depends on, and usage records — SHALL
NOT be touched.

An undelivered dispatch record older than a fortnight SHALL keep its ledger truth (the
goal really never landed, and status views SHALL keep saying so) but SHALL NO LONGER
drive live judgments about the pane id it names.

#### Scenario: A reboot's reissued pane ids do not inherit old records

- **WHEN** a restore completes on a freshly started server
- **THEN** pane-keyed records naming panes that no longer exist are removed before any
  conversation is brought back

#### Scenario: A failed pane query reaps nothing

- **WHEN** the live-pane set cannot be read
- **THEN** no records are removed

#### Scenario: Conversation records survive

- **WHEN** the sweep runs at the moment every pane is gone
- **THEN** resume and usage records are untouched, because restore reads them precisely
  then

#### Scenario: An ancient undelivered dispatch stops steering its pane

- **WHEN** a dispatch record that never delivered is older than the shelf life and its
  pane id has been reissued
- **THEN** it no longer triggers a screen-classification of that pane, while status views
  still report the dispatch as undelivered
