## MODIFIED Requirements

### Requirement: OpenSpec workflow skills are provisioned across agent harnesses

The repository SHALL ship all six OpenSpec 2.x workflow skills (`openspec` with its `references/`, `openspec-propose`, `openspec-apply-change`, `openspec-archive-change`, `openspec-explore`, `openspec-sync-specs`) as regular files (never symbolic links) in both `.agents/skills/` and `.claude/skills/`, generated from the pinned `@fission-ai/openspec` CLI version, and SHALL document harness-neutral `openspec` CLI invocations in `AGENTS.md`. CI SHALL fail if any required OpenSpec skill or reference file is missing, empty, or symlinked under `.agents/skills/` or `.claude/skills/`.

#### Scenario: An OpenSpec skill is missing from the cross-harness directory

- **WHEN** one of the six required OpenSpec skills (or a required `openspec/references/` file) is absent, empty, or symlinked under `.agents/skills/` or `.claude/skills/`
- **THEN** the conformance gate fails and names the offending skill path
