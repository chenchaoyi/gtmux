# Tasks — single-source HQ home seeding

- [x] 1. `seedHQHome`: single-source, idempotent, no second full doc (fresh → AGENTS.md + import; AGENTS-only → add import; legacy CLAUDE.md → leave, no zombie)
- [x] 2. `isClaudePointer` + `hqPolicyWarning`; `cmdHQ` prints the warning on a redundant/broken layout
- [x] 3. Tests: no-zombie over legacy CLAUDE.md; AGENTS-only adds pointer; warn on two-full-docs / dangling import; fresh idempotent
- [x] 4. Spec delta: `supervisor-agent` "Launchable supervisor session" seed behavior + scenarios
- [ ] 5. Docs: `CLAUDE.md` repo-guide seed note (if it cites the old two-file behavior)
- [ ] 6. `make check` + `check-design` green; PR
- [ ] 7. Archive after merge
