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

### Requirement: Resume only panes that were running an agent when the layout was saved

`gtmux restore` SHALL decide whether a restored pane gets an agent conversation from the
tmux-resurrect SAVE — the `pane_current_command` / `pane_full_command` it recorded for
that pane — and NOT from the presence of a resume record at the pane's locator.

A resume record is written by the agent's hooks and is never pruned, so it attests only
that an agent ran in that pane at SOME point; treating it as evidence of a live session
made every pane that had ever hosted an agent a permanent resume target. A pane the save
shows sitting at an interactive shell SHALL NOT be resumed into, and a pane with no line
in the save SHALL NOT be resumed into either (restore did not create it). When the save
records a program that is not an agent, the pane SHALL NOT be resumed into. When the save
records something that cannot be identified — no full command, and a
`pane_current_command` that names no known agent (a live Claude Code pane reports its
VERSION there) — the resume SHALL be allowed and the ambiguity logged: silently failing
to bring back a conversation that really was running is the worse failure.

When NO save can be read at all, the gate SHALL be disabled rather than deny every
resume, so a missing save degrades to the previous behavior and not to silence.

The parser SHALL handle both of tmux-resurrect's pane-line layouts. resurrect re-reads its
own dump with a tab-delimited shell `read`, and because a tab is IFS whitespace a pane with
an EMPTY title loses that field, shifting every later field left by one; such lines carry
the pane's pid where the command belongs and a full command computed from the wrong pid.
The directory field's `:` prefix distinguishes the layouts, and the full command on a
shifted line SHALL be discarded.

#### Scenario: A shell pane is left alone

- **WHEN** a restored pane's saved line shows an interactive shell, and a resume record
  exists at that pane's locator from an agent that ran there in the past
- **THEN** restore types nothing into that pane and logs that the save shows a shell

#### Scenario: A live agent pane comes back

- **WHEN** a restored pane's saved line shows an agent command (`claude --resume <id>`)
- **THEN** restore resumes that pane's conversation

#### Scenario: An empty pane title does not disguise a shell

- **WHEN** a saved pane line lost its title field (fields shifted left), so the fixed
  command column holds the pane's pid
- **THEN** the parser reads the shifted layout and still sees the pane's real command,
  so a shell pane on such a line is refused like any other

#### Scenario: Unidentifiable but running

- **WHEN** a saved pane line records no usable full command and a
  `pane_current_command` that matches no known agent
- **THEN** restore allows the resume and logs that the evidence was unclear

### Requirement: Recover the conversation id from the saved command line

When a pane the save shows running an agent has NO resume record at its locator,
`gtmux restore` SHALL take the conversation id from the agent command line the save
recorded for that pane (e.g. `claude --resume <id>`), before falling back to any
directory-based guess. The resume record SHALL still take precedence when present: hooks
keep it current across `/clear` and compaction, whereas the saved command line only knows
the id the process was launched with.

#### Scenario: Missing record, id in the save

- **WHEN** a restored pane was running `claude --resume <id>` at save time and no resume
  record exists for its locator
- **THEN** restore resumes `<id>` into it rather than leaving the pane empty

#### Scenario: The record disagrees with the saved command line

- **WHEN** both a resume record and a saved command-line id exist for the same pane
- **THEN** the record wins

### Requirement: The cwd fallback requires layout-position agreement

When a restored pane's exact locator has no saved resume record and the save recorded no
conversation id for it, `gtmux restore` MAY recover a conversation by a fallback, but ONLY
from a record that shares BOTH the pane's working directory AND its window.pane layout
position (the coordinates tmux-resurrect preserves across a reboot). A directory match
alone SHALL NOT authorize a resume. This is required because many restored panes are plain
shells (editors, extra terminals) that merely sit inside a project directory without ever
having hosted an agent; a directory-only fallback injected a historical conversation into
every such pane, so a single session came back showing several agent conversations that
were never running. Position agreement is the evidence that the restored pane is the same
pane that hosted the conversation (its session having only been renamed); a pane at a
position no agent ever occupied SHALL recover nothing.

The fallback runs only for panes that already passed the liveness gate above, and because
it is a GUESS — the record it matches was saved under a DIFFERENT locator — it SHALL
additionally refuse records that were last updated more than a fortnight before the save.
The directory compared SHALL be the one the SAVE recorded for the pane when available,
which is the pre-reboot truth; the pane's live working directory is `/` whenever the
directory failed to restore.

#### Scenario: A bare shell pane sharing a project directory is not injected

- **WHEN** a restored pane at a window.pane position that never hosted an agent sits
  in a directory where some other pane once ran a conversation
- **THEN** restore resumes nothing into it, rather than injecting a historical
  conversation from that directory

#### Scenario: A renamed session still resumes at its position

- **WHEN** a pane's exact locator no longer matches (its session was renamed) but a
  saved record shares the pane's directory and its window.pane position
- **THEN** restore recovers that conversation into the pane

#### Scenario: A long-abandoned record is not guessed into a pane

- **WHEN** the only record matching a pane's directory and position was last updated
  more than a fortnight before the save was taken
- **THEN** restore recovers nothing into that pane

### Requirement: The restore plan shows what restore will actually do

`gtmux restore --plan` (and its `--json` form, the menu bar's source for the expandable
restore row) SHALL list, per saved session, only the agent conversations that the real
restore would relaunch — applying the same liveness gate and the same
record → saved-command-line → directory-guess order. A conversation the plan lists but
restore would not resume (or the reverse) is a defect: the plan is a promise about what
the restore is about to do.

#### Scenario: The plan does not advertise a phantom

- **WHEN** a saved pane was an interactive shell but a resume record exists at its locator
- **THEN** the plan lists no conversation for that pane

#### Scenario: The plan counts the same conversations restore resumes

- **WHEN** a plan is built from a save and a restore is then driven from that same save
- **THEN** the conversations listed and the conversations resumed are the same set

### Requirement: Only a real conversation may claim a pane's resume record

The system SHALL NOT let a session take over a pane's resume record unless that session
is a real conversation — one whose log yields at least one parsed turn. This is required
because an agent runs parts of its own machinery as SEPARATE sessions in the SAME pane
(a slash command such as `/usage` gets its own session id and fires the same hooks), so a
differing session id is not evidence that the pane changed hands. Recording such a stub
makes the pane's chat history read as empty and, worse, makes restore relaunch the stub
after a reboot instead of the conversation that was running — losing access to the work
through the very record meant to preserve it. A session already recorded for the pane
SHALL always be able to update its own record, and a real conversation SHALL still be
able to take over a pane, so a genuine handover is unaffected. When the recorded
conversation's log no longer exists, the system SHALL allow the claim regardless, since a
record that can never be resumed must not pin the pane.

#### Scenario: A command stub does not steal the pane

- **WHEN** a slash command runs as its own session in a pane already recorded for a live
  conversation
- **THEN** the pane's record still names the live conversation, so chat history and
  restore both continue to point at the work

#### Scenario: A genuine handover still works

- **WHEN** the user starts a new conversation in a pane that was recorded for an older one
- **THEN** the new conversation becomes the pane's record

#### Scenario: The same conversation keeps reporting

- **WHEN** the session already recorded for a pane fires another hook
- **THEN** its record is updated, without inspecting any log

#### Scenario: A record pointing at a vanished conversation is replaceable

- **WHEN** the recorded conversation's log no longer exists on disk
- **THEN** another session may claim the pane, rather than the pane staying bound to a
  conversation that can never be resumed

### Requirement: Restore is defined by a contract, and the contract is executed

The system SHALL define what restore preserves as an enumerated contract, and SHALL verify
each machine-verifiable dimension automatically by saving a known session topology,
destroying the server, restoring, and asserting that dimension. Verification SHALL drive a
real terminal multiplexer rather than a substitute, because the failures being guarded
against arise in the interaction between the system, the save/restore tool and a live
server — the part a substitute removes. Verification SHALL be confined to a private server
and a private save location, so it can never affect the operator's own sessions or saves.
The contract SHALL state which dimensions are NOT machine-verifiable rather than omitting
them, so its coverage is not overstated.

The enumerated dimensions are: the set of sessions; each session's window order and names;
each window's pane layout; each pane's working directory; and the active window and pane
per session. The order of a host terminal's own windows is part of the contract but is NOT
machine-verifiable and remains a manual check.

#### Scenario: A dimension regresses

- **WHEN** a change breaks any machine-verifiable dimension of restore
- **THEN** the verification fails and names the dimension that broke

#### Scenario: Verification cannot harm live work

- **WHEN** the verification runs on a machine with live sessions and existing saves
- **THEN** neither is read, modified, or destroyed

### Requirement: A running server missing saved sessions has them restored

When a server is already running and ANY saved session is absent from it, the system SHALL
restore the absent ones. It SHALL NOT require that ALL saved sessions be absent: after a
restart something routinely starts one session on its own, and requiring all-absent made
that the condition under which the remaining sessions were never recovered — permanently,
once the next autosave recorded their absence. Restoring alongside live sessions is safe
because the save/restore tool creates only what does not already exist.

#### Scenario: One session came back on its own

- **WHEN** a running server holds one saved session and is missing the others
- **THEN** the missing ones are restored

#### Scenario: Nothing is missing

- **WHEN** every saved session is already live
- **THEN** no restore is driven

### Requirement: Restore returns you to the window and pane you were on

The system SHALL restore each session's active window and that window's active pane. It
SHALL NOT rely on a mechanism that requires an attached client, because restore runs
headlessly — before any client attaches — and such a mechanism silently does nothing,
leaving every session on its first window regardless of where the user was working. Where a
save records no active window for a session, the system SHALL leave that session alone
rather than selecting a default, since selecting the first window is indistinguishable from
the failure being corrected.

#### Scenario: A session was left on a later window

- **WHEN** a session was saved with a non-first window active
- **THEN** after restore that window is active, and within it the pane that was active

#### Scenario: The save records no active window

- **WHEN** a save marks no active window for a session
- **THEN** no window is selected for it

### Requirement: The layout backstop never runs alongside the autosaver

The system SHALL save the tmux layout itself ONLY when the periodic autosaver is not
armed. When the autosaver is armed a second saver is not redundancy but a RACE: both run
the same save routine over the same files, and concurrent runs have produced duplicate
save files and a truncated pane-contents archive — the system corrupting the very save
restore depends on. Corrupting the save is strictly worse than the staleness the backstop
guards against. The system SHALL also invoke the save script in its QUIET mode, because
its default mode paints a progress message into the multiplexer's message line on every
attached client and forks an extra process to animate it, producing recurring on-screen
noise with no visible cause.

#### Scenario: The autosaver is armed

- **WHEN** the periodic save trigger is present
- **THEN** the system does not save the layout itself

#### Scenario: The autosaver is missing

- **WHEN** the periodic save trigger is absent and the save has gone stale
- **THEN** the system saves the layout itself, without printing to the message line
