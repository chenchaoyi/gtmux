## ADDED Requirements

### Requirement: Hermetic real-handler contracts
Synthetic Go HTTP handler outputs SHALL be compared with independent RN and Web fake responses in ordinary CI, including types, empty values, pagination and permission errors. Fakes SHALL NOT replay expected responses to manufacture agreement.

#### Scenario: A fake drifts from history semantics
- **WHEN** a fake renumbers source turns or returns a different stale-cursor status
- **THEN** its hermetic contract check fails without a running user server

### Requirement: Isolated and measurable test execution
Android journeys SHALL retain their full assertions and permit capability blocks with explicit setup and blocked dependencies. Browser process reuse SHALL preserve per-case contexts and fake servers. Timings SHALL distinguish action and evidence/setup cost. Test-only CI narrowing SHALL prove non-product ownership and fail closed on uncertain dependencies.

#### Scenario: A block setup fails
- **WHEN** setup fails before its selected UI checks
- **THEN** dependent checks are blocked with the first cause and independent blocks remain eligible

#### Scenario: A browser context changes storage
- **WHEN** the next case uses the same browser process
- **THEN** its fresh context and fixture cannot observe the previous case state
