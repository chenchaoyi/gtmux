# Environment Doctor Specification

## Purpose

Map each gtmux feature to the tmux/terminal/hook prerequisite it needs and report
what works and what to fix — so a new user can set up the whole environment
without hunting. Read-only by default; opt-in to apply fixes.

## Requirements

### Requirement: Read-only grouped health check

The system SHALL, on `gtmux doctor`, run a read-only check grouped by concern
(tmux, restore, terminal, agents+notifications), each row a status glyph + label
+ value + a short "why", and a summary tally. It SHALL change nothing.

#### Scenario: Healthy environment

- **WHEN** `gtmux doctor` runs with everything configured
- **THEN** it prints the grouped checks all ✓ and exits 0, changing nothing

#### Scenario: Blocking issue

- **WHEN** a required prerequisite is missing (e.g. tmux absent, or set-titles
  not configured for focus/restore)
- **THEN** that row is marked blocking and the command exits non-zero

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

The system SHALL, via `--fix`, also install the Claude hook, clone missing tmux
plugins (TPM/resurrect/continuum), and offer to wire Codex's notify — so
`doctor --fix` is the one-stop setup. It SHALL print guidance for what it cannot
safely do (install tmux, install the app).

#### Scenario: Codex notify is single-slot

- **WHEN** a non-gtmux `notify` already exists in `~/.codex/config.toml`
- **THEN** the system warns and asks before replacing it (default no), and never
  replaces it under `--yes`
