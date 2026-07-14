## MODIFIED Requirements

### Requirement: Launchable supervisor session

The system SHALL provide `gtmux hq` (中控): it creates — or focuses, when one
already runs — a dedicated tmux session running the user's coding agent (Claude
by default, per existing agent profiles) with its working directory set to the
persistent supervisor home (`~/.config/gtmux/hq/`). The playbook SHALL be
gtmux-OWNED and VERSION-TRACKED: AGENTS.md is the canonical FULL playbook (the
cross-agent convention Codex/Cursor/Amp read natively) carrying a machine-parseable
VERSION marker, and CLAUDE.md is a one-line `@AGENTS.md` import so Claude reads the
SAME content with no two-doc drift (`--agent`/`GTMUX_HQ_AGENT` pick which agent runs).
User PERSONALIZATION SHALL live in a separate seed-once `LOCAL.md` that the generated
AGENTS.md `@`-imports (reaching Claude through the CLAUDE.md→AGENTS.md→LOCAL.md chain);
gtmux SHALL create `LOCAL.md` once from a template and SHALL NEVER overwrite it. On
`gtmux hq`, when the SHIPPED playbook version is newer than the installed one, the
system SHALL UPGRADE the managed AGENTS.md: back up the prior file to
`AGENTS.md.bak-v<old>` FIRST, regenerate at the new version, and print a one-line
notice. When the versions match it SHALL be idempotent (no rewrite). An existing
AGENTS.md with NO version marker SHALL be treated as version 0 and MIGRATED once via
the same backup-then-regenerate path, with the notice directing the user to move any
personal edits into `LOCAL.md`. The situation board (`notes/board.md`) and knowledge
base (`knowledge/*`) SHALL remain seed-if-absent and SHALL NOT be touched by an
upgrade. A legacy full CLAUDE.md (pre-AGENTS.md convention) SHALL remain authoritative
and SHALL NOT get a zombie AGENTS.md dropped beside it. `gtmux hq` SHALL WARN — rather
than silently proceed — when it detects a redundant layout (a full CLAUDE.md alongside
AGENTS.md) or a broken one (a CLAUDE.md `@AGENTS.md` import while AGENTS.md is missing).
The seeded playbook teaches the supervisor to loop — read `gtmux digest --json`, judge,
drill into a pane (`tmux capture-pane`) only when warranted, drive via `gtmux send`,
report to the user with a token-usage section ALWAYS included in status reports (the
per-type rollup + any `usage_warn` sessions, via `gtmux usage --json`) — and the
supervisor's accumulated knowledge persists across sessions.

#### Scenario: Fresh home seeds the managed playbook + LOCAL.md

- **WHEN** `gtmux hq` runs and `~/.config/gtmux/hq/` has NO policy file
- **THEN** a version-stamped AGENTS.md (the full playbook), a CLAUDE.md `@AGENTS.md`
  import, and an empty `LOCAL.md` template are generated, and a tmux session starts the
  agent there

#### Scenario: A newer shipped version upgrades the playbook

- **WHEN** `gtmux hq` runs and the installed AGENTS.md version is older than the shipped
  `hqPlaybookVersion`
- **THEN** the prior AGENTS.md is backed up to `AGENTS.md.bak-v<old>`, AGENTS.md is
  regenerated at the shipped version, and `gtmux hq` prints a one-line upgrade notice

#### Scenario: Matching version is idempotent

- **WHEN** `gtmux hq` runs and the installed AGENTS.md version equals the shipped one
- **THEN** AGENTS.md is left unchanged (no rewrite) and no notice is printed

#### Scenario: LOCAL.md is never overwritten

- **WHEN** `gtmux hq` upgrades the playbook and the user has content in `LOCAL.md`
- **THEN** `LOCAL.md` is left exactly as the user wrote it, and its content still reaches
  the agent via the AGENTS.md import

#### Scenario: A legacy unversioned AGENTS.md is migrated once

- **WHEN** `gtmux hq` runs and the home holds an AGENTS.md with no version marker
- **THEN** it is backed up to `AGENTS.md.bak-v0`, regenerated at the shipped version, and
  the notice directs the user to move any personal edits into `LOCAL.md`

#### Scenario: The board and knowledge base survive an upgrade

- **WHEN** `gtmux hq` upgrades the playbook
- **THEN** `notes/board.md` and every `knowledge/*` file are left untouched

#### Scenario: A legacy full CLAUDE.md gets no zombie AGENTS.md

- **WHEN** `gtmux hq` runs and the home already holds a full CLAUDE.md but no AGENTS.md
- **THEN** the CLAUDE.md is left untouched and NO AGENTS.md is created beside it

#### Scenario: A redundant or broken layout warns

- **WHEN** the home has a full CLAUDE.md alongside AGENTS.md, or a CLAUDE.md
  `@AGENTS.md` import while AGENTS.md is missing
- **THEN** `gtmux hq` prints a warning naming the redundant/broken doc and how to
  resolve it, rather than silently proceeding
