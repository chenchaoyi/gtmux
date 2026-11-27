# docs-conformance Specification

## Purpose

The CI gate that keeps documentation's factual claims honest about the code —
and the explicit boundary of what a gate can and cannot know.

## Requirements

### Requirement: A documented example of a code-produced format is rendered by that code

A documented example of a code-generated format SHALL be marked as a rendered region
naming a registry id, and CI SHALL fail when the region's content differs from what that
registry entry's builder actually produces — printing both. The registry SHALL live in Go beside the builders, so
it cannot compile against a builder whose shape changed. A registered id with no
corresponding region SHALL also fail (a dead entry is drift too). Regions are OPT-IN: an
unmarked example is deliberately unchecked. CI SHALL only report; a separate command
rewrites the regions from the code. The check and the rewrite SHALL find marked regions in
every Markdown document under `docs/`, `api/` and `openspec/specs/`, at any depth, and in
the README pair; `openspec/changes/` is not searched (a proposal quotes what it changes,
and the archive is history).

#### Scenario: A marked region in a nested document is checked

- **WHEN** a document under `docs/design/` (or its translation) carries a marked region
- **THEN** the docs check verifies it like any other, and the rewrite command rewrites it

#### Scenario: A builder changes and the doc does not

- **WHEN** the wake-line builder's format changes and a documented example still shows the
  old rendering
- **THEN** the docs check fails, naming the region and showing both the documented and the
  produced line

#### Scenario: A doc example is regenerated rather than hand-copied

- **WHEN** a contributor runs the docs-fix command after a builder change
- **THEN** the marked regions are rewritten from the code and CI passes, with no line
  hand-transcribed

#### Scenario: A registered example loses its region

- **WHEN** a marked region is deleted from the doc but its registry entry remains
- **THEN** the check fails rather than silently checking nothing

### Requirement: The wake vocabulary is complete wherever it is taught

Every wake class the code can emit SHALL appear both in the user-facing class table
(`docs/cli.md`) and in the seeded HQ playbook — the two surfaces that teach the vocabulary
to the human and to HQ respectively. CI SHALL fail on any class missing from either, so a
newly added class cannot reach a reader (or a supervisor) as an unexplained glyph. A class
deliberately not surfaced SHALL be exempted through an explicit allowlist, mirroring the
existing command-registry check's `HIDDEN`.

#### Scenario: A new wake class is added and documented nowhere

- **WHEN** a wake class constant is added without a row in the class table or a mention in
  the playbook
- **THEN** the docs check fails, naming the class and the surface it is missing from

#### Scenario: An internal class is exempt on purpose

- **WHEN** a class is listed in the allowlist
- **THEN** the check passes without requiring it to be documented

### Requirement: Retired vocabulary cannot return

A token retired by a shipped change SHALL be denied in the documentation tree — a
superseded wake format, a deleted package path, a description a change explicitly killed —
and each denylist entry SHALL name the change that retired it, so the list reads as an
audit trail and an entry can be retired in turn.

#### Scenario: A superseded format reappears

- **WHEN** documentation reintroduces a retired token (a superseded wake-line prefix, a
  deleted package path)
- **THEN** the docs check fails, naming the change that retired it

### Requirement: The gate states its own boundary

The conformance gate SHALL NOT be presented as verifying that documentation is TRUE. Its
scope is claims with a machine-readable source: rendered formats, enumeration membership,
and retired tokens. Whether a doc's prose correctly describes behavior — for example that
a completion wake fires for any session rather than only a dispatched task — SHALL remain
an explicit review-gate item, stated as such where the rules are recorded, so a green gate
is never read as a reviewed doc.

#### Scenario: Prose remains a review-gate item

- **WHEN** documentation misdescribes behavior in a sentence with no machine-readable
  source
- **THEN** the gate does not claim to catch it, and the repo's rules record that judgment
  as a reviewer's responsibility

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

The repository SHALL ship all six OpenSpec 2.x workflow skills (`openspec` with its `references/`, `openspec-propose`, `openspec-apply-change`, `openspec-archive-change`, `openspec-explore`, `openspec-sync-specs`) as regular files (never symbolic links) in both `.agents/skills/` and `.claude/skills/`, generated from the pinned `@fission-ai/openspec` CLI version, and SHALL document harness-neutral `openspec` CLI invocations in `AGENTS.md`. CI SHALL fail if any required OpenSpec skill or reference file is missing, empty, or symlinked under `.agents/skills/` or `.claude/skills/`.

#### Scenario: An OpenSpec skill is missing from the cross-harness directory

- **WHEN** one of the six required OpenSpec skills (or a required `openspec/references/` file) is absent, empty, or symlinked under `.agents/skills/` or `.claude/skills/`
- **THEN** the conformance gate fails and names the offending skill path
