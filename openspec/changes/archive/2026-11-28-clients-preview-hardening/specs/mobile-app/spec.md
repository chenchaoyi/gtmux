## ADDED Requirements

### Requirement: Foreground-gated polling cadences and bounded SSE retries
Mobile screens SHALL gate periodic reads on navigation focus and `AppState === 'active'` with `AbortSignal`. `HQConsoleView` SHALL poll resources at 30s and activity at 6s (skipping hidden feeds in `workspaceMode`), while `HQWorkspaceView` pauses periodic polling on the console. `AgentsProvider` SHALL close errored SSE streams before backoff, cap consecutive `401`/`403` retries at two, keep SSE open on `inactive`, and throttle owner `share` reads.

#### Scenario: Backgrounded or unauthorized mobile session
- **WHEN** a screen loses focus, enters `background`, or receives two consecutive `401`/`403` SSE responses
- **THEN** unfocused/backgrounded pollers and errored SSE streams stop immediately, `inactive` keeps SSE open, and confirmed `401`/`403` retries halt
