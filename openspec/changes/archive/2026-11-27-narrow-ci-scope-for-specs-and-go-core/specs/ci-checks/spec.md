## MODIFIED Requirements

### Requirement: Conservative check selection

PR and main-push CI SHALL always run policy/docs, strict specs, bilingual pairing, workflow validation and CI guard tests. Only recognized docs, `.openspec.yaml` metadata and policy-only guard scripts MAY omit platform suites. Known module and scoped Go package changes SHALL run complete corresponding suites and explicit downstream consumers; mixed changes SHALL use the union. Shared contracts/assets, dependencies, CI selector changes, symlinks or unknown/unreadable diffs SHALL run all suites and explain the selection.

#### Scenario: Prose changes

- **WHEN** a PR changes only recognized Markdown documents, archived/active `.openspec.yaml` metadata, or policy-only guard scripts
- **THEN** policy and documentation gates run while native/platform suites can skip

#### Scenario: Pure Go CLI or supervisor package changes

- **WHEN** a PR changes non-HTTP Go packages (such as `cmd/gtmux` or `internal/hq`) without touching shared contract, server, artifact, relay, or dependency files
- **THEN** policy/docs, complete Go (`test`), and `cross-build` suites run while unrelated iOS, Android, menu-bar, Web, and Worker suites can skip (whereas `internal/server`, `internal/artifact`, and `relay` include their Web, RN, menu-bar, or relay consumers)

#### Scenario: Uncertain or mixed impact

- **WHEN** diff resolution fails, a new unknown component is added, `scripts/ci.py` or `api/contract.md` changes, or a shared contract file is renamed into docs (both rename paths and deletions are classified, and manual CI runs all suites)
- **THEN** all platform suites are selected

#### Scenario: Module changes

- **WHEN** a PR changes menu-bar Swift code and Web styles
- **THEN** Swift, complete Go and Web suites run, while unrelated phone/Worker suites can skip
