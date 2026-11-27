# docs-conformance Specification Delta

## ADDED Requirements

### Requirement: Repository agent instructions have one harness-agnostic source of truth

Repository contributor instructions and workflow rules SHALL live in a single root
`AGENTS.md` file rather than a harness-specific file. Harness-specific entry files
(`CLAUDE.md`, `GEMINI.md`) SHALL be thin regular-file `@AGENTS.md` import pointers
(never symbolic links, preserving CI's symlink ban) and SHALL NOT duplicate substantive
rules or headings. The CLI command registry conformance check SHALL read `AGENTS.md`.
CI SHALL fail if `AGENTS.md` is missing, if either pointer is missing, symlinked, or
diverges from a thin `@AGENTS.md` pointer, or if a user-facing CLI command is missing
from `AGENTS.md`.

#### Scenario: A harness-specific entry file duplicates rules instead of pointing to AGENTS.md

- **WHEN** `CLAUDE.md` or `GEMINI.md` replaces the thin `@AGENTS.md` pointer with duplicated sections or a symbolic link
- **THEN** the conformance gate fails and identifies the offending pointer file

#### Scenario: Any agent harness reads repository rules

- **WHEN** a contributor opens the repository in Claude Code, Codex, Antigravity (`agy`), Gemini CLI, OpenCode, Kimi Code, Cursor, or Copilot
- **THEN** the harness resolves the same canonical workflow and architecture rules from `AGENTS.md`

### Requirement: OpenSpec workflow skills are provisioned across agent harnesses

The repository SHALL ship the core OpenSpec workflow skills (`openspec-propose`,
`openspec-apply-change`, `openspec-archive-change`, `openspec-explore`,
`openspec-sync-specs`) in both the cross-harness `.agents/skills/` standard directory
and `.claude/skills/` (alongside `.claude/commands/opsx/` for Claude Code), generated
from the pinned OpenSpec CLI version, and SHALL document harness-neutral `openspec` CLI
invocations in `AGENTS.md` for harnesses without skill auto-discovery. CI SHALL fail if
any required OpenSpec skill is missing from either `.agents/skills/` or `.claude/skills/`.

#### Scenario: An OpenSpec skill is missing from the cross-harness directory

- **WHEN** one of the five core OpenSpec skills is absent or empty under `.agents/skills/` or `.claude/skills/`
- **THEN** the conformance gate fails and names the missing skill and directory
