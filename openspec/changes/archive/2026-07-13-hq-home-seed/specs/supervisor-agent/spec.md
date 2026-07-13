# supervisor-agent Specification

## MODIFIED Requirements

### Requirement: Launchable supervisor session

The system SHALL provide `gtmux hq` (中控): it creates — or focuses, when one
already runs — a dedicated tmux session running the user's coding agent (Claude
by default, per existing agent profiles) with its working directory set to the
persistent supervisor home (`~/.config/gtmux/hq/`). The home SHALL be seeded
SINGLE-SOURCE and IDEMPOTENT: AGENTS.md is the canonical FULL playbook (the
cross-agent convention Codex/Cursor/Amp read natively) and CLAUDE.md is a one-line
`@AGENTS.md` import so Claude reads the SAME content with no two-doc drift
(`--agent`/`GTMUX_HQ_AGENT` pick which agent runs). A home that already holds a
policy file SHALL be treated as already-seeded: the system SHALL NOT add a SECOND
full policy doc and SHALL NOT overwrite any policy file. In particular a legacy full
CLAUDE.md SHALL remain authoritative and SHALL NOT get a zombie AGENTS.md dropped
beside it; when only AGENTS.md exists, the cheap CLAUDE.md `@AGENTS.md` import MAY be
added. `gtmux hq` SHALL WARN — rather than silently proceed — when it detects a
redundant layout (a full CLAUDE.md alongside AGENTS.md) or a broken one (a CLAUDE.md
`@AGENTS.md` import while AGENTS.md is missing). The seeded playbook teaches the
supervisor to loop — read `gtmux digest --json`, judge, drill into a pane
(`tmux capture-pane`) only when warranted, drive via `gtmux send`, report to the
user with a token-usage section ALWAYS included in status reports (the per-type
rollup + any `usage_warn` sessions, via `gtmux usage --json`) — and the
supervisor's accumulated knowledge persists across sessions.

#### Scenario: Fresh home seeds single-source

- **WHEN** `gtmux hq` runs and `~/.config/gtmux/hq/` has NO policy file
- **THEN** AGENTS.md (the full playbook) and CLAUDE.md (the `@AGENTS.md` import) are
  generated, and a tmux session starts the agent there

#### Scenario: A legacy full CLAUDE.md gets no zombie AGENTS.md

- **WHEN** `gtmux hq` runs and the home already holds a full CLAUDE.md but no AGENTS.md
- **THEN** the CLAUDE.md is left untouched and NO AGENTS.md is created beside it

#### Scenario: An AGENTS.md-only home gains only the import

- **WHEN** `gtmux hq` runs and the home holds AGENTS.md but no CLAUDE.md
- **THEN** a one-line CLAUDE.md `@AGENTS.md` import is added — never a second full copy

#### Scenario: Relaunch reuses, never clobbers

- **WHEN** `gtmux hq` runs while a supervisor session is already live
- **THEN** it focuses the existing session instead of spawning a second, and every
  existing (possibly user-edited) policy file is left untouched

#### Scenario: A redundant or broken layout warns

- **WHEN** the home has a full CLAUDE.md alongside AGENTS.md, or a CLAUDE.md
  `@AGENTS.md` import while AGENTS.md is missing
- **THEN** `gtmux hq` prints a warning naming the redundant/broken doc and how to
  resolve it, rather than silently proceeding
