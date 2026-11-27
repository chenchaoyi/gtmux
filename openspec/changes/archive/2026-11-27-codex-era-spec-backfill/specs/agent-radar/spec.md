## ADDED Requirements

### Requirement: Bounded v2 participant profile storage and cross-generation reconciliation

New hook observations SHALL write private snapshots under `participant-profiles/v2/` (0700 dir, 0600 files) using at most 256 reusable shard locks in `locks/` after process inspection. Readers SHALL merge legacy and v2 observations for the same agent, home, and session so newer model records govern model-dependent options and newer permission records govern permission modes, while preserving effort resets across generations.

#### Scenario: Model switches away and back across hook generations
- **WHEN** a newer observation selects a different model and later switches back after an effort reset
- **THEN** stale model-dependent options (`effort`, `variant`, `thinking`, `thinking_level`, `thinking_budget`, `context`, `model_alias`) from the older generation do not overwrite the newer model's defaults
- **AND** a newer effort observation may cross generations for matching models, while a legacy effort at or before the reset stays cleared even if a legacy hook later refreshes the model

#### Scenario: Concurrent hook updates under bounded shard locks
- **WHEN** many distinct sessions record hook profile observations
- **THEN** process inspection runs before acquiring a shard lock and writes reuse at most 256 shard locks under `participant-profiles/v2/locks/`

### Requirement: Fresh radar snapshot sharing and per-pass pane frame reuse

On every periodic status tick, `gtmux serve` SHALL run a fresh radar scan and publish it to the bounded HTTP list-read cache without letting the REST TTL suppress SSE ticks or an older concurrent REST load overwrite a fresher refresh. Within one gather pass, ordinary status probes SHALL reuse one captured frame per pane without retaining frames across polls, while wait-withdrawal and active approval checks SHALL capture a fresh frame.

#### Scenario: REST read arrives shortly after an SSE tick
- **WHEN** an authenticated HTTP client requests the agent list within the snapshot TTL after a serve tick
- **THEN** it receives the cached snapshot published by the serve tick without triggering a duplicate full radar gather, while standalone CLI reads keep their direct fresh path
- **AND** the next SSE tick still performs a fresh gather independently of the REST TTL

#### Scenario: Multiple ordinary probes inspect the same pane in one gather
- **WHEN** a single gather pass evaluates multiple ordinary status checks (hook-free frame, Codex screen error/wait, idle composer) for the same pane
- **THEN** those checks reuse one per-pass captured frame for that pane without keeping it across polls
- **AND** approval verification and wait-withdrawal final checks read a fresh pane frame rather than reusing the cached status frame
