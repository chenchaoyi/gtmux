# Tasks — agent-session-fidelity

## 1. Codex session-to-pane correlation

- [x] Add session-id-to-live-pane lookup using current resume bindings, requiring agent and
  cwd agreement.
- [x] Make cwd fallback require one unambiguous live Codex pane, including when the inherited
  pane happens to share that cwd.
- [x] Add tests for stale inherited pane, same-cwd parallel panes, explicit session match,
  and no-match behavior.

## 2. HQ session lineage keeps agent identity

- [x] Add additive structured predecessor/successor agent identity to HQ replacement audit
  records without changing legacy summary text.
- [x] Read structured identity first and retain conservative legacy inference.
- [x] Cover Claude→Codex handoff and retain the legacy inference test.

## 3. Agent capability checks and Codex parser contract

- [x] Conformance-check declared transcript parser, hook installer/display, and dedicated
  semantics wiring against the registry.
- [x] Add a sanitized Codex fixture for current/legacy prompt shapes and tool calls.
- [x] Update English and Chinese agent onboarding docs with the capability contract and
  fixture workflow, keeping the two versions aligned.
- [x] Specify per-turn agent attribution and current/legacy Codex prompt records in the
  chat-transcript contract.

## 4. Codex plan metadata

- [x] Carry optional `plan_type` from Codex rollout rate limits to the limits window JSON.
- [x] Add parser/API tests for present and absent plan type; keep existing renderers compatible.
- [x] Document `plan_type` in the JSON API contract.
- [x] Scope incremental usage counters by agent/session and preserve old counters through
  first-read migration.

## 5. Verification

- [x] Run focused Go tests for hook, transcript, events, limits, agents, and app packages.
- [x] Run the repository-required `make check`.
- [x] Run `scripts/check-design.sh` with its pinned OpenSpec 1.10.0 package available.
- [x] Validate this change with the locally installed OpenSpec CLI.
- [x] Review `git diff` to ensure existing user changes remain intact and paired docs agree.
