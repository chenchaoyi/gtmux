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

#### Scenario: Autosave trigger present

- **WHEN** the continuum plugin is installed and `status-right` contains the `continuum_save` trigger
- **THEN** doctor reports the autosave as OK (shown as `installed`, one consistent
  install-state word with the hooks/plugins/app — not a bespoke `armed`/`wired`)

#### Scenario: Autosave trigger missing

- **WHEN** the continuum plugin is installed but `status-right` does not contain the trigger
- **THEN** doctor flags it with a recommendation to add the `continuum_save.sh` interpolation to `status-right`

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

#### Scenario: Several versions crossed

- **WHEN** a user updates across more than one release
- **THEN** the summary covers all of them, newest first

#### Scenario: More than fits

- **WHEN** more changes exist than the summary shows
- **THEN** it says how many remain and how to see them

#### Scenario: Notes unavailable

- **WHEN** the notes cannot be fetched
- **THEN** the update reports success and prints no summary and no error

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
whether it is up to date with the CLI.

#### Scenario: App section detail

- **WHEN** the menu-bar app is installed
- **THEN** a "Menu-bar app" section reports its version + on-disk path, and flags it if it is
  behind the CLI

### Requirement: Terminal landscape beyond the host

Beyond the host terminal (the one gtmux drives), `gtmux doctor` SHALL detect which OTHER
known terminals are installed and mark each as supported (a gtmux driver exists →
focus/restore/new work) or sensed-only (hosts tmux + agents fine, but no driver).

#### Scenario: Other terminals listed

- **WHEN** terminals besides the host are installed (e.g. iTerm2 + Apple Terminal)
- **THEN** doctor lists them, each marked supported or sensed-only

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
