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
- **THEN** doctor reports the autosave as armed (OK)

#### Scenario: Autosave trigger missing

- **WHEN** the continuum plugin is installed but `status-right` does not contain the trigger
- **THEN** doctor flags it with a recommendation to add the `continuum_save.sh` interpolation to `status-right`

