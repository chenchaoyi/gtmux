## ADDED Requirements

### Requirement: Antigravity hooks isolate subagents, record launch chains, and preserve concrete models

Antigravity hook handling SHALL ignore subagent callbacks where `rootConversationId` differs from `conversationId`, classify raw hook events before detachment so `PreInvocation` and `PostToolUse` capture verified launch-process chains, skip `"modelName": "auto"` so concrete `PLANNER_RESPONSE.model` facts remain intact, and strip standalone `<SYSTEM_MESSAGE>` injections from user prompts.

#### Scenario: Mid-session Antigravity hook captures launch processes and ignores auto model placeholder

- **WHEN** a root `agy` `PreInvocation` hook fires with `"modelName": "auto"` on a conversation whose log recorded `"model": "gemini-4-carbon-medium"`
- **THEN** pre-detach observation records the verified launch process chain and keeps `"gemini-4-carbon-medium"` as the session's `model` fact

#### Scenario: Antigravity subagent hook does not overwrite parent session state

- **WHEN** an `agy` hook arrives with non-empty `rootConversationId` different from `conversationId`
- **THEN** the hook emits its required JSON stdout and exits without mutating the parent pane's resume binding, turn state, or participant profile

#### Scenario: System wakeup messages and search_web calls normalize cleanly in transcripts

- **WHEN** an `agy` transcript contains a `<SYSTEM_MESSAGE>`-only `USER_INPUT` entry and a `search_web` tool call
- **THEN** the system wakeup does not open a user turn and the `search_web` step records `Action: "web_search"` with its query detail
