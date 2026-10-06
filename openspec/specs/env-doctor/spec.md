# Environment Doctor Specification

## Purpose

Map each gtmux feature to the tmux/terminal/hook prerequisite it needs and report
what works and what to fix — so a new user can set up the whole environment
without hunting. Read-only by default; opt-in to apply fixes.

## Requirements

### Requirement: Read-only grouped health check

The system SHALL, on `gtmux doctor`, run a read-only check grouped by concern
(tmux, restore, terminal, agents+notifications), each row a status glyph + label
+ value + a short "why", and a summary tally. The check itself SHALL change
nothing. When it finds improvable or blocking rows AND is running on an
interactive terminal (a TTY), it SHALL, after the report, OFFER to apply the
fixes inline (the same consent-gated per-step flow as `--fix`), so the user does
not have to re-invoke with `--fix`; declining the offer, or running off a TTY,
keeps the command read-only and prints the `--fix` hint instead.

The system SHALL announce each check section on stderr before evaluating that
section when running in a terminal. `--progress` SHALL enable the same announcements
when output is redirected. The optional Homebrew update suggestion SHALL NOT delay
the report indefinitely.

During the agent and notification section, `gtmux doctor` SHALL announce each
subcheck before running it when progress is enabled. The last line SHALL name
the currently running hook, chat-binding, or hook-traffic probe.

#### Scenario: Healthy environment

- **WHEN** `gtmux doctor` runs with everything configured
- **THEN** it prints the grouped checks all ✓ and exits 0, changing nothing

#### Scenario: Blocking issue

- **WHEN** a required prerequisite is missing (e.g. tmux absent, or set-titles
  not configured for focus/restore)
- **THEN** that row is marked blocking and the command exits non-zero

#### Scenario: Offer to fix inline on a TTY

- **WHEN** `gtmux doctor` (no `--fix`) finds improvable/blocking rows on an
  interactive terminal
- **THEN** after the report it asks whether to fix now, and on assent walks the
  same consent-gated fix flow; declining keeps it read-only

#### Scenario: Non-interactive stays read-only

- **WHEN** `gtmux doctor` runs off a TTY (piped / CI) with improvable rows
- **THEN** it does NOT prompt and changes nothing, printing the `gtmux doctor
  --fix` hint

#### Scenario: A check is slow

- **WHEN** `gtmux doctor` runs in a terminal, or with `--progress` when piped
- **THEN** it writes each check section to stderr before running that section,
  so the last visible stage identifies where it is waiting; stdout remains the
  grouped report
- **AND** the optional Homebrew update probe uses cached metadata without an
  automatic update and stops after a bounded wait, leaving the tmux version row
  available even if Homebrew is stuck

#### Scenario: A probe stalls

- **WHEN** a doctor probe is slow
- **THEN** the last announced section identifies the current stage
- **AND** a stuck Homebrew update lookup times out, allowing the report to finish

#### Scenario: Agent probe is slow

- **WHEN** a chat-binding or hook-traffic probe takes a long time
- **THEN** stderr identifies that probe without changing the report on stdout

### Requirement: Locale / UTF-8 health check and fix

The system SHALL check that the tmux server's locale is UTF-8 — resolving the
effective charset by POSIX precedence (`LC_ALL` > `LC_CTYPE` > `LANG`) — and report
a non-UTF-8 locale as a problem, because a non-UTF-8 tmux mangles multi-byte agent
glyphs (e.g. the tool-call marker) so the radar's agent classifier can yield
nothing. Under `--fix`, it SHALL inject a UTF-8 `LANG` into the managed tmux server
environment (e.g. `set-environment -g LANG en_US.UTF-8`) within the marked managed
block, idempotently.

#### Scenario: Non-UTF-8 locale flagged

- **WHEN** the resolved locale charset is not UTF-8
- **THEN** `gtmux doctor` reports it as a problem with a short "why" (agent glyphs
  can be mangled, breaking detection)

#### Scenario: Fix sets a UTF-8 LANG

- **WHEN** `gtmux doctor --fix` applies the locale fix
- **THEN** a UTF-8 `LANG` is set in the managed tmux server environment, written in
  the managed block and idempotent across runs

#### Scenario: The server's locale, not gtmux's own

- **WHEN** the tmux server's global environment and the environment `gtmux doctor` itself
  runs in disagree about UTF-8
- **THEN** the row follows the server's, which is what new panes start with; with no server
  running it reads the current shell's and says so, and when the server cannot be asked it
  says it did not check instead of guessing

#### Scenario: LANG is outranked

- **WHEN** the server's `LC_ALL` or `LC_CTYPE` is set to a non-UTF-8 locale
- **THEN** the row names that variable, and `--fix` does not set `LANG` (it would change
  nothing) but says which variable to change, without clearing one the user set

### Requirement: Apply fixes with per-change consent

The system SHALL, on `gtmux doctor --fix`, walk the recommended fixes one at a
time, explaining each change and asking before applying it (`--yes` applies all;
off a TTY it skips rather than mutating silently).

#### Scenario: Confirm each change

- **WHEN** `gtmux doctor --fix` runs interactively
- **THEN** each step prints what it changes and why and prompts before applying;
  declining a step skips only that step

#### Scenario: Conservative tmux.conf edits

- **WHEN** a tmux.conf change is applied
- **THEN** it is written inside a clearly marked managed block, the file is backed
  up first, only options the running tmux is actually missing are written, and
  managed lines are merged (never dropped) across runs

#### Scenario: Live tmux settings cannot be read

- **WHEN** tmux is installed but doctor cannot read its running global options
- **THEN** doctor reports a blocking connection check with the underlying error,
  rather than reporting unreadable title, history, or restore options as unset
- **AND** `--fix` exits non-zero before applying changes, with guidance to start
  tmux or run from a terminal that can access its socket

#### Scenario: A saved configuration cannot be applied live

- **WHEN** a managed config write succeeds but any live tmux command fails
- **THEN** `--fix` retains the saved config, reports the failed command and its
  underlying error, and exits non-zero without claiming that step succeeded
- **AND** it still attempts the other commands in that step

#### Scenario: Checks remain after a fix pass

- **WHEN** a fix pass leaves recommended or blocking checks
- **THEN** its summary shows each check's current value and guidance without
  claiming that all remaining checks are unfixable or are not configuration
- **AND** blocking checks or earlier application failures keep the exit non-zero

### Requirement: Folds in hook + plugin setup

The system SHALL, via `--fix`, also install the Claude hook, wire Codex via its
ADDITIVE hooks system (`~/.codex/hooks.json` + `features.hooks`), clone missing tmux
plugins (TPM/resurrect/continuum), and — after consent — install the menu-bar app —
so `doctor --fix` is the one-stop setup. It SHALL print guidance for the one thing it
can't safely automate: installing tmux.

#### Scenario: Codex wired additively, notify untouched

- **WHEN** `doctor --fix` wires Codex
- **THEN** it adds gtmux to Codex's hooks system (`hooks.json` + `features.hooks`) and
  NEVER writes or replaces `notify` in `~/.codex/config.toml` (the old
  single-slot notify-replace step was removed in #317)

#### Scenario: features.hooks enabled under an existing [features] table

- **WHEN** `doctor --fix` wires Codex and `~/.codex/config.toml` already has a
  `[features]` table (without `hooks = true`)
- **THEN** it WRITES `hooks = true` under that table (inserting the key, or flipping
  an existing `hooks = false`), preserving the rest of the file — it does NOT merely
  print guidance, so a follow-up `gtmux doctor` reports Codex wired
- **AND** if it still cannot enable `features.hooks`, the fix reports that honestly
  (it does not claim success)

#### Scenario: Un-wired Codex is a recommended improvement

- **WHEN** `~/.codex` exists (Codex is in use) but the gtmux hook is not wired
- **THEN** `gtmux doctor` marks the Codex-hook row as a recommended improvement (not
  a neutral note), so it counts toward "to improve" and the fix flow offers it

#### Scenario: Installs the app, guides for tmux

- **WHEN** the menu-bar app is missing and the user consents
- **THEN** `doctor --fix` installs it (via the same installer as `gtmux update`)
- **AND** if tmux is missing, it only PRINTS how to install it (never runs a package
  manager), since that isn't safe to automate

#### Scenario: Homebrew app is already installed

- **WHEN** a complete Gtmux.app bundle exists in `/Applications` but not `~/Applications`
- **THEN** `doctor --fix` does not offer to install the app again

### Requirement: An agent's hooks are reported as complete, not merely present

"Installed" and "installed COMPLETELY" are different facts, and a hooks file only ever
answers the first on its own. Nothing rewrites an agent's hooks config on update — that
is `install hooks`, which nobody re-runs unprompted — so a file written by an older
gtmux keeps working, keeps reading installed, and silently lacks every event added since.

For each agent whose hooks gtmux installs, the system SHALL compare the events the
config carries a GTMUX entry for against the events gtmux currently registers for that
agent, and SHALL report a shortfall by naming the missing events and the command that
reinstalls them. This applies to Claude, whose events come from gtmux's own list, as
much as to the agents driven by an installer registry — a check written for one shape
leaves the other blind, which is how Claude's two compaction events stayed missing while
its row read a plain green "installed".

Presence SHALL be judged by whose hook it is, not by whether the event is configured:
another tool's hook on the same event does not deliver anything to gtmux.

A config carrying NO gtmux entry at all SHALL NOT be reported as incomplete — that is
"not installed", which the row already says in its own words.

A shortfall SHALL be repairable by `--fix`, on the same terms as a missing install: it is
the same act, it preserves every other tool's hooks, and it backs the file up first. What
`--fix` CANNOT do is load the new events into sessions already running — that needs a
restart, which would end a live conversation — so the report SHALL say so rather than
implying the events are now flowing.

#### Scenario: --fix tops up an incomplete install

- **WHEN** `gtmux doctor --fix` runs against a hooks config that carries gtmux entries for
  only some of the events gtmux registers
- **THEN** it offers to add the missing ones, preserves the other tools' hooks, and says
  the sessions already running must be restarted to load them

#### Scenario: A hooks file that predates the current event set

- **WHEN** an agent's hooks config carries gtmux entries for only some of the events
  gtmux now registers
- **THEN** the row reports how many are missing, names them, and names the reinstall
  command

#### Scenario: Someone else's hook on the same event

- **WHEN** the config has an entry for an event gtmux wants, but the command belongs to
  another tool
- **THEN** that event counts as missing for gtmux

#### Scenario: No gtmux entry anywhere

- **WHEN** the config carries no gtmux hook at all
- **THEN** the row reports it as not installed, and says nothing about missing events

### Requirement: Remote-access readiness check

The system SHALL include a "Remote access" section in the doctor report that checks
whether `cloudflared` (the default anywhere-tunnel client) is installed, and via
`--fix` SHALL offer to install it (`brew install cloudflared`) or otherwise point at
the manual install — so `gtmux tunnel` is one consent away, consistent with the other
fixers. This is advisory: a missing `cloudflared` does not block LAN/self-hosted use.

#### Scenario: cloudflared missing

- **WHEN** `doctor` runs and `cloudflared` is not installed
- **THEN** the "Remote access" row flags it, and `--fix` offers to install it (with
  consent) or prints the manual install command

### Requirement: Check the resurrect autosave is armed

`gtmux doctor` SHALL, in its "Restore after reboot" section and only when the
tmux-continuum plugin is installed, check that the running tmux `status-right` carries
continuum's save trigger (the `continuum_save.sh` interpolation continuum relies on to
autosave). When the trigger is missing it SHALL recommend adding it, because a custom
`status-right` without it silently disables autosave — the save goes stale and a reboot
restores an ancient snapshot.
`doctor --fix` SHALL offer to append the installed plugin's absolute-path trigger
without replacing the existing status text, and SHALL persist an idempotent guard
in the backed-up managed config block. It SHALL apply the change to the running
tmux and report failure if that cannot be verified. Existing and duplicated
triggers SHALL remain untouched.

When an installed continuum script exists but the running tmux status-right has
no save trigger, `doctor --fix` SHALL offer to add the script's absolute-path
trigger while retaining the existing status text. It SHALL back up the config,
persist a guarded append in the managed block, apply it live, and verify one
trigger is present. Existing and duplicate triggers SHALL remain untouched.

#### Scenario: Autosave trigger present

- **WHEN** the continuum plugin is installed and `status-right` contains the `continuum_save` trigger
- **THEN** doctor reports the autosave as OK (shown as `installed`, one consistent
  install-state word with the hooks/plugins/app — not a bespoke `armed`/`wired`)

#### Scenario: Autosave trigger missing

- **WHEN** the continuum plugin is installed but `status-right` does not contain the trigger
- **THEN** doctor flags it with a recommendation to add the `continuum_save.sh` interpolation to `status-right`

#### Scenario: Fix a missing trigger

- **WHEN** the installed save script exists, status-right has no trigger, and the user accepts `doctor --fix`
- **THEN** the fix preserves existing status text, adds one absolute-path trigger now, and persists a guarded append in the managed config block
- **AND** reloading the config does not add a second trigger, including when continuum already injected one

#### Scenario: Missing save trigger

- **WHEN** the installed continuum plugin has a save script but the current status-right has no trigger
- **THEN** `doctor --fix` offers to append one absolute-path trigger after consent
- **AND** sourcing the config again does not append a duplicate

#### Scenario: Live apply fails

- **WHEN** the config is written but the running tmux cannot be armed
- **THEN** the fixer reports failure instead of claiming autosave is enabled

### Requirement: A duplicated autosave trigger is reported

The doctor SHALL report when the status line carries the periodic-save trigger more than
once, and SHALL say how many. A duplicate makes every save interval run the save that many
times, for as long as the configuration stands, with nothing on screen to indicate it —
checking only for PRESENCE cannot see it. The cause SHALL be named in the advice, because
it is not guessable: the save plugin decides whether to inject by looking for its own
ABSOLUTE path, so a trigger written by hand with a `~` path does not match and a second,
absolute copy is appended. A trigger written in ANY path form SHALL count, so a
correctly-armed setup using one spelling is never reported as unarmed.

#### Scenario: A hand-written and an auto-injected trigger coexist

- **WHEN** the status line carries a `~`-path save trigger and an absolute-path one
- **THEN** the doctor reports two triggers and explains that the save runs twice

#### Scenario: One trigger, either spelling

- **WHEN** the status line carries exactly one save trigger, in any path form
- **THEN** the doctor reports autosave as armed

### Requirement: An update reports what changed, in the user's terms

After a successful self-update the system SHALL summarise what changed, and the summary
SHALL be written for the user rather than derived from commit subjects — a commit subject
addresses whoever reads the diff, and identifiers such as revision hashes, change numbers
and author handles are noise to someone who only wants to know whether something they rely
on moved. The summary SHALL aggregate EVERY version crossed, not only the newest, since a
user several releases behind is exactly the one for whom describing one release would be a
lie of omission. It SHALL be brief, and point to a fuller listing for the remainder. It
SHALL be SILENT when it has nothing to say or cannot fetch the notes: this runs after the
install already succeeded, and an error about it reads as though the update itself failed.
A release whose author wrote no user-facing note SHALL contribute nothing rather than
having one invented for it.
After a successful install, `gtmux update` SHALL remind the user to run
`gtmux doctor` to check the local setup. `--check` and failed installs SHALL NOT
print that reminder; update SHALL NOT run the full doctor probe automatically.

After a successful install, `gtmux update` SHALL print a localized reminder to
run `gtmux doctor`. It SHALL not run the full doctor probe automatically or print
the reminder for `--check` or a failed install.

#### Scenario: Several versions crossed

- **WHEN** a user updates across more than one release
- **THEN** the summary covers all of them, newest first

#### Scenario: More than fits

- **WHEN** more changes exist than the summary shows
- **THEN** it says how many remain and how to see them

#### Scenario: Update completes

- **WHEN** `gtmux update` successfully installs a release
- **THEN** it prints a localized prompt to run `gtmux doctor`, even if no release notes could be fetched

#### Scenario: Notes unavailable

- **WHEN** the notes cannot be fetched
- **THEN** the update reports success and prints no summary and no error

#### Scenario: Update succeeds without release notes

- **WHEN** an update succeeds but release notes are unavailable
- **THEN** the doctor reminder still appears

### Requirement: Doctor reports a left-behind sleep setting and fixes only gtmux's own

The doctor SHALL check whether the machine's sleep-disable setting is applied while no
live gtmux server-mode heartbeat exists, and SHALL report it as a finding, because that state
is otherwise invisible to the user (the setting persists across reboot and leaves no
trace in the default power-settings listing when off). When gtmux's ownership stamp shows
the setting is gtmux's own, `doctor --fix` SHALL offer to restore sleep under the existing
per-change consent rule. When there is no gtmux ownership stamp, the finding SHALL be
report-only with the manual command shown, and `--fix` SHALL NOT change it. The doctor
SHALL also report whether the privileged sleep guard is installed and healthy.

#### Scenario: An orphan gtmux left behind

- **WHEN** the sleep-disable setting is applied, gtmux's stamp owns it, and no live
  heartbeat exists
- **THEN** doctor reports that the Mac will not sleep and why, and `--fix` restores sleep
  after the usual per-change consent

#### Scenario: A setting gtmux does not own

- **WHEN** the sleep-disable setting is applied but carries no gtmux ownership stamp
- **THEN** doctor reports it as informational with the manual command to undo it, and
  `--fix` leaves it untouched

### Requirement: Reports the gtmux install (versions + config) first

`gtmux doctor` SHALL open with a "gtmux" section that reports the CLI's version and its
on-disk path, the installed menu-bar app's version, and whether `~/.config/gtmux/config.json`
parses. When the app's version differs from the CLI's it SHALL flag the drift (recommended,
with `run gtmux update`), because a CLI ahead of the app is a real, confusing state
(`gtmux update` refreshes a stale app). A menu-bar bundle whose `CFBundleShortVersionString`
carries a leading `v` SHALL be normalized so a current app is never mis-read as behind.

#### Scenario: CLI ahead of the app

- **WHEN** the CLI version is newer than the installed menu-bar app's version
- **THEN** the "gtmux" section shows both versions and flags the app as behind, pointing at
  `gtmux update`

#### Scenario: Malformed config

- **WHEN** `~/.config/gtmux/config.json` exists but is not valid JSON
- **THEN** doctor flags it as invalid (settings are silently ignored otherwise); an absent
  config reports "defaults" (neutral)

### Requirement: Consistent install-state vocabulary

Rows that report an install-like state (plugins, hooks, the resurrect autosave, the
menu-bar app) SHALL use the SINGLE word `installed`, not per-check synonyms (`armed`,
`wired`). Runtime toggles (tmux options) keep `on`. This is a presentation invariant so the
report reads uniformly.

#### Scenario: Codex hook and autosave read as installed

- **WHEN** the Codex hook is wired and the resurrect autosave trigger is present
- **THEN** both report the value `installed` (matching the Claude Code hook / plugins / app)

### Requirement: Menu-bar app as its own section

`gtmux doctor` SHALL report the menu-bar app in a dedicated "Menu-bar app" section (not
folded into "Agents & notifications") showing its install state, version, on-disk path, and
whether it is up to date with the CLI. Doctor, its fixer, and update SHALL use the same
install-location search (`~/Applications` before `/Applications`) so they cannot
disagree about whether the app is present.

Doctor and `doctor --fix` SHALL agree on the installed app path, searching
`~/Applications` before `/Applications`. An incomplete bundle lacking
`Contents/Info.plist` SHALL not hide a complete installation in the other path.

#### Scenario: App section detail

- **WHEN** the menu-bar app is installed
- **THEN** a "Menu-bar app" section reports its version + on-disk path, and flags it if it is
  behind the CLI

#### Scenario: App installed by Homebrew

- **WHEN** Gtmux.app is present only in `/Applications`
- **THEN** doctor reports it as installed and the fixer skips the app-install step

### Requirement: Terminal landscape beyond the host

Beyond the host terminal (the one gtmux drives), `gtmux doctor` SHALL detect which OTHER
known terminals are installed and mark each as supported (a gtmux driver exists →
focus/restore/new work), best-effort (a driver exists but cannot fully drive the
terminal — today Warp: restore/new work, focus is exact only for gtmux-opened tabs), or
sensed-only (hosts tmux + agents fine, but no driver). The host-terminal row SHALL be
equally honest: when the host is only best-effort driven, its row SHALL say what works
and what degrades instead of claiming full support.

#### Scenario: Other terminals listed

- **WHEN** terminals besides the host are installed (e.g. iTerm2 + Apple Terminal)
- **THEN** doctor lists them, each marked supported, best-effort, or sensed-only

#### Scenario: Warp marked best-effort, never fully supported

- **WHEN** Warp is the host terminal (or installed besides it)
- **THEN** its row states the best-effort limits (restore/new supported; focus
  best-effort — Warp has no tab scripting) rather than a plain "supported"

### Requirement: Live serve, phone uploads, and HQ health

`gtmux doctor` SHALL, additionally: report whether a local `gtmux serve` is actually
LISTENING (the phone's endpoint — installed-tunnel-without-a-serve still can't answer the
phone); surface the phone image-upload staging dir (`~/.local/share/gtmux/uploads`) with its
size + file count and clear it via `--fix` (the images were already delivered); and, when an
HQ home exists, report HQ's health in detail — its home, the situation board's freshness, and
the knowledge base's shape (topic count + pending-distill queue) — alongside the existing
consumption/distill/self-check cadences.

#### Scenario: Serve not running

- **WHEN** no local server is listening on the serve port
- **THEN** doctor reports the serve as not running (the phone can't reach this Mac)

#### Scenario: Phone uploads cleanup

- **WHEN** the phone-upload dir holds staged images and the user runs `doctor --fix`
- **THEN** after consent it clears them and reports how many were removed

#### Scenario: HQ health detail

- **WHEN** an HQ home exists
- **THEN** the "HQ" section reports the board's freshness and the knowledge base's shape in
  addition to the maintenance cadences

### Requirement: Reports whether panes can be told apart on screen

`doctor` SHALL report two things about pane identity, and SHALL measure the OUTCOME rather
than the presence of a setting:

- whether the terminal tab names the panes behind it, counted per WINDOW by the NAME itself
  (does it carry an id), never by the `automatic-rename` option alone — renaming a window
  (by hand, or by `gtmux spawn` naming its own) turns that option off permanently, but a
  window gtmux named still backfills its own pane ids into the name, so the option being off
  does not mean the window is unnamed. When some windows carry no ids of their own — a
  genuine hand-rename with nothing to show for it — it SHALL say so rather than reporting a
  bare fraction;
- whether PLAIN panes have a title that says anything, counted per pane, excluding agent
  panes (an agent writes its own title, so counting them would report a healthy fleet made
  of rows the user cannot tell apart).

A title SHALL be treated as saying nothing when it is empty, the machine's host name, a
filesystem path, or the pane's own command — the four shapes measured on a real fleet.

`--fix` SHALL OFFER the corresponding configuration and never apply it silently, SHALL show
the exact lines first, SHALL back up the file it edits, and SHALL write into a marked block.
For the shell hook it SHALL choose the file a tmux pane actually sources — a login shell —
and SHALL only create a new startup file when none of the candidates exist.

#### Scenario: A fleet whose tabs cannot name their panes

- **WHEN** `automatic-rename-format` carries no `#{pane_id}`
- **THEN** the row is reported as improvable and `--fix` offers the format plus the
  `pane-exited` refresh hook

#### Scenario: Windows the setting cannot reach

- **WHEN** the format is set but some windows were renamed by hand with no ids of their own
- **THEN** the row reports how many windows carry ids AND how many are hand-named, instead
  of a ✓ that contradicts what the user sees on screen

#### Scenario: A gtmux-named window still counts as carrying ids

- **WHEN** `gtmux spawn` names a window (turning `automatic-rename` off for it) and
  backfills the window's own pane ids into the name
- **THEN** that window counts toward `named`, not toward the hand-named count — the option
  being off is not, by itself, evidence the window carries no ids

#### Scenario: Applying the fix shows on screen immediately

- **WHEN** `--fix` applies the format
- **THEN** every auto-named window is refreshed at once, rather than waiting for each window's
  next activity; hand-named windows are left alone

#### Scenario: Plain panes all read as their command

- **WHEN** no plain pane has an informative title
- **THEN** the row is reported as improvable and `--fix` offers the shell hook, written to
  the startup file a login shell reads

### Requirement: Offers only what the running tmux understands

`doctor` SHALL check the running tmux's version before offering a configuration line whose
option or feature name postdates some releases, and SHALL NOT offer it when the running
tmux would reject it. An unknown or unparseable version SHALL be treated as too old: a
suggestion is offered only when it is known to work.

#### Scenario: Clickable links on a tmux that supports them

- **WHEN** tmux is 3.4 or newer and `terminal-features` does not claim `hyperlinks`
- **THEN** the row reports that tmux is dropping hyperlinks, and `--fix` offers the
  `terminal-features` line

#### Scenario: The same machine on an older tmux

- **WHEN** tmux predates the `hyperlinks` feature
- **THEN** the row says which version it needs and `--fix` offers nothing, because writing
  an unknown feature name into the user's config is a startup error on a line they did not
  write

### Requirement: doctor reports knowledge distribution per agent

`gtmux doctor` SHALL show a "knowledge sync" row per supported agent with one of: in
sync, missing, stale, hand-edited. `--fix` SHALL refresh missing and stale blocks and
SHALL leave hand-edited ones with a message.

#### Scenario: A new agent is installed after the last sync

- **WHEN** an agent's global instruction file exists without the block
- **THEN** doctor reports it missing and `--fix` installs the block

### Requirement: Doctor checks the logs and everything else that grows

`gtmux doctor` SHALL include a `Logs` section with: the log store's size against its cap
and its oldest day against the retention window (flagged when over the cap, or when days
older than retention plus two remain, meaning cleanup is not running); a runaway writer
(flagged when a day passed the in-day cap in the last 7 days, naming the component and
event); recent errors (counts of `error` entries in the last 24 hours by component and
event); file modes (flagged when anything gtmux keeps is readable by another account, with
a count and an example path); every other growing store against its written bound and any
retired file still on disk; and, when present, credential backups earlier migrations left
in the config root. `gtmux doctor --fix` SHALL run the log cleanup, narrow modes and remove
retired files, and SHALL remove the credential backups only after asking.

#### Scenario: Cleanup has stopped

- **WHEN** the log store holds a day file 45 days old and retention is 30 days
- **THEN** the log store row is flagged as not being cleaned, and `gtmux doctor --fix`
  deletes the expired days and records an `act.cleanup` entry

### Requirement: Doctor reports the tunnel from its status

`gtmux doctor`'s remote-access section SHALL include a tunnel row read from
`status/tunnel.json` when a tunnel is set up: the backend, whether it is connected, since
when, and the last error, or that no current status exists. The cloudflared row SHALL say
it is not used when the backend is Direct. The serve row SHALL say what this Mac has
established and no more: with no tunnel set up, that serve is listening for the local
network; with a tunnel that reports itself connected, that serve is listening and this
Mac's own health check of the tunnel's public address passed, and that the phone's own
network is where reachability is confirmed. It SHALL NOT state that the phone can connect
from any network.

#### Scenario: Direct is down

- **WHEN** the backend is Direct and `status/tunnel.json` reports `down` with an error
- **THEN** the tunnel row is flagged and shows the error, and the serve row does not say
  that the phone can reach this Mac

### Requirement: Doctor can pack a bug report

`gtmux doctor --bundle <path>` SHALL write an archive of `logs/`, `status/`, the doctor
report and version information, SHALL exclude the journal and user data unless
`--with-events` is given, and SHALL print what it included.

#### Scenario: Default bundle

- **WHEN** the user runs `gtmux doctor --bundle out.tgz`
- **THEN** `out.tgz` contains no `events.jsonl` and no uploads, and the command lists the
  files it packed

### Requirement: Doctor reports Claude Code's renderer

When `~/.claude` exists, `gtmux doctor` SHALL report which renderer Claude Code uses, because
its fullscreen renderer runs in the alternate screen, where tmux keeps no scrollback and every
gtmux surface that reads a pane sees one screen. The row SHALL be decided from the settings
file's `tui` key and its `env` block, falling back to doctor's own environment, read as Claude
reads them: booleans are `1/true/yes/on` and `0/false/no/off`; a true
`CLAUDE_CODE_DISABLE_ALTERNATE_SCREEN` forces the classic renderer first; `CLAUDE_CODE_NO_FLICKER`
forces fullscreen when true and classic when false; a `tui` value other than `default` or
`fullscreen` counts as unset. It SHALL also use whether running Claude panes are in the alternate
screen, since Claude chooses fullscreen by itself on a fresh install when `tui` is unset; with no
Claude pane to check, an unset `tui` SHALL be reported as not pinned rather than classic. A
settings file that exists but cannot be read or parsed SHALL be reported as such, with nothing
inferred from it. `doctor --fix` SHALL offer to write `"tui": "default"` only when the file is
readable, there is evidence of fullscreen and there is no explicit env choice; it SHALL first
back the file up to its own backup (so a hook step's backup in the same run keeps the original),
stop and report failure if that backup cannot be written, and keep every other value as written.
It SHALL NOT type into a running session; it SHALL say that running sessions keep their renderer
until restarted (or `/tui default` in each) and that scrollback a fullscreen session never gave
tmux cannot be recovered.

#### Scenario: A reinstall switched Claude to fullscreen

- **WHEN** `tui` is unset and three running Claude panes are in the alternate screen
- **THEN** the row warns "fullscreen (3 of 3 sessions)"
- **AND** `doctor --fix` writes `"tui": "default"` and keeps hooks, env and other keys as they were

#### Scenario: Pinned, but sessions started before it

- **WHEN** `tui` is `default` and two running Claude panes are still in the alternate screen
- **THEN** the row warns that they must be restarted or switched with `/tui default`
- **AND** `doctor --fix` changes nothing

#### Scenario: The user forced fullscreen by env

- **WHEN** the settings `env` sets `CLAUDE_CODE_NO_FLICKER` to `yes`
- **THEN** the row names the variable and `doctor --fix` does not override it

#### Scenario: A false NO_FLICKER beats a fullscreen setting

- **WHEN** `tui` is `fullscreen` and `CLAUDE_CODE_NO_FLICKER` is `off`
- **THEN** the row is classic and says the env overrides the setting

#### Scenario: The settings file cannot be read

- **WHEN** `~/.claude/settings.json` is not valid JSON
- **THEN** the row says it cannot be read and `doctor --fix` leaves it untouched

#### Scenario: Classic renderer, nothing pinned

- **WHEN** `tui` is unset and Claude panes are running, none in the alternate screen
- **THEN** the row is healthy and says a fresh install or update may switch it

#### Scenario: No Claude session to check

- **WHEN** `tui` is unset and no Claude pane is running
- **THEN** the row says it is not pinned and does not call it classic

