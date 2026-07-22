# Session Restore Specification

## Purpose

Bring your tmux workspace back after the terminal quits or the machine reboots —
layout, directories, and screen text — driving tmux-resurrect/continuum
deterministically so a large layout is never lost or silently replaced.
## Requirements
### Requirement: Reattach after quitting the terminal

The system SHALL, when the tmux server is still alive but the terminal was quit,
open one terminal tab per session and attach them (see terminal-jump's restore).

#### Scenario: Tabs gone, sessions alive

- **WHEN** the terminal is quit and `gtmux restore` is run
- **THEN** each tmux session gets an attached terminal tab

#### Scenario: Every unattached session gets its own tab

- **WHEN** restore opens tabs for the unattached sessions
- **THEN** it opens a NEW tab for EVERY one of them and never silently drops a
  session by reusing the current tab for the first (which previously orphaned the
  alphabetically-first session when the current process wasn't a reusable terminal)

### Requirement: Restore tabs in the recorded order

The system SHALL restore terminal tabs in the user's last recorded tab order
rather than tmux's alphabetical `list-sessions` order. The always-on menu-bar app
records the live tab→session order on a slow timer (`gtmux save-tab-order` →
`~/.local/share/gtmux/tab-order`, plain text, one session per line); restore
replays it.

#### Scenario: Tabs come back in your arrangement

- **WHEN** restore opens tabs and a tab-order record exists
- **THEN** sessions present in the record are opened in that order, and any
  sessions not in the record follow in their existing relative order

#### Scenario: No record yet

- **WHEN** no tab-order record exists (e.g. the menu-bar app never ran)
- **THEN** restore falls back to the default order, unchanged

### Requirement: Restore after reboot via resurrect

The system SHALL, when the tmux server is down, start it and DRIVE tmux-resurrect
restore explicitly (run-shell, in-server context), waiting for the restore to
complete rather than racing a fixed timeout.

#### Scenario: Large layout takes time

- **WHEN** restoring a large saved layout that takes longer than a fixed timeout
- **THEN** the system waits until restored sessions settle, then proceeds

### Requirement: Recover when an empty server is already up after reboot

The system SHALL recover the saved layout even when a tmux server is ALREADY
running but missing it — the post-reboot trap where a reopened terminal tab (or
anything) started an empty server before `gtmux restore` ran, which would
otherwise skip the restore. It SHALL drive the restore only when NONE of the
saved sessions are live (to avoid duplicating a normal reattach).

#### Scenario: Empty server, saved sessions missing

- **WHEN** a server is up whose sessions do not include any of the saved
  sessions, and a real saved layout exists
- **THEN** the system drives the tmux-resurrect restore into the running server

#### Scenario: Sessions already present

- **WHEN** a server is up that already has the saved sessions (a normal reattach
  after the terminal quit)
- **THEN** the system does NOT re-restore, avoiding duplicate sessions

### Requirement: Never overwrite a good save

The system SHALL, if a saved layout exists but did not restore, refuse to keep a
bare fallback session as if all is well (which continuum would then autosave over
the good save) — it warns and points at the save instead.

#### Scenario: Restore failed but a save exists

- **WHEN** a save with real sessions exists but the restore produced nothing
- **THEN** the system warns loudly, does not overwrite the save, and surfaces its
  path for recovery

### Requirement: Repair a poisoned `last` pointer

The system SHALL, before booting the server, repair a tmux-resurrect `last`
symlink that points at an empty save (resurrect repoints `last` on any content
change, even a 0-byte race), pointing it at the newest save that has a layout.

#### Scenario: `last` points at an empty save

- **WHEN** `last` is missing or resolves to a save with no window/pane lines
- **THEN** it is repointed to the newest timestamped save that has a layout; a
  `last` already resolving to a real layout is left untouched

### Requirement: Restore scrollback when configured

The system SHALL restore each pane's scrollback (screen text) when
`@resurrect-capture-pane-contents` is on, with `history-limit` controlling depth.

#### Scenario: Scrollback snapshot

- **WHEN** capture-pane-contents is enabled and a restore runs
- **THEN** each pane's prior scrollback comes back as a snapshot

### Requirement: Warn when restoring from a stale save

`gtmux restore` SHALL warn the user when the tmux-resurrect save it is about to restore
is older than one day. The warning names how old the save is and that sessions created
since will not come back, and is written both to the user (stderr) and the restore log.
A fresh or absent save produces no warning. This keeps a silently-broken autosave (a save
that stopped updating) from restoring an ancient snapshot without any signal.

#### Scenario: Stale save at reboot restore

- **WHEN** the tmux server is down and `gtmux restore` resolves a resurrect save whose contents are more than a day old
- **THEN** it prints a prominent warning that the saved layout is N old and newer sessions won't restore, and logs the same, before restoring

#### Scenario: Fresh save restores quietly

- **WHEN** the resolved save is recent (updated within the last day)
- **THEN** no staleness warning is printed and restore proceeds normally

### Requirement: serve backstops the resurrect save

`gtmux serve` SHALL, on its slow tick, trigger a tmux-resurrect save ITSELF whenever a
tmux server is up AND the last resurrect save is stale (older than a short interval), so
that save freshness does not depend on tmux-continuum's autosave being correctly armed.
When the last save is already fresh (continuum is working) it SHALL do nothing. The save
SHALL run as a direct subprocess with `$TMUX` and a robust PATH (NEVER `tmux run-shell`,
which runs in the server's minimal-PATH environment, exits non-zero, and can poison the
`last` pointer with an empty save), and the `last` pointer SHALL be repaired afterward if
a save wrote an empty file. Concurrent backstop saves SHALL be prevented (single-flight).

#### Scenario: Backstop fires when continuum is dead

- **WHEN** serve's slow tick runs, a tmux server is up, and the last resurrect save is older than the backstop interval
- **THEN** serve triggers a resurrect save as a direct subprocess, keeping the save fresh even though continuum never saved

#### Scenario: Backstop is a no-op when continuum is healthy

- **WHEN** serve's slow tick runs and the last resurrect save was updated within the backstop interval
- **THEN** serve does not trigger a save (no duplicate work)

### Requirement: Resume a conversation from the directory it is filed under

When relaunching an agent conversation, `gtmux restore` SHALL resume it from the directory
the conversation is actually filed under, not merely the working directory last observed
for the pane — an agent may change directory mid-session, and resuming from the moved-to
directory fails to find the conversation. For an agent whose transcript store is known
(Claude Code), the system SHALL locate the transcript by session id and take the session's
recorded working directory from the transcript itself rather than decoding the store's
directory name (that encoding is lossy). If no transcript exists for the session id, the
system SHALL SKIP the resume and record why, rather than running a command that can only
report a missing conversation. Agents whose store cannot be inspected SHALL be unaffected.

#### Scenario: The agent changed directory during the session

- **WHEN** a conversation was started in one directory, the agent later changed into a subdirectory, and restore relaunches it
- **THEN** the resume runs from the directory the conversation is filed under, so the conversation is found

#### Scenario: The conversation no longer exists

- **WHEN** a resume record names a session id with no transcript on disk
- **THEN** restore skips that resume and logs the reason, leaving a usable pane instead of an error message

