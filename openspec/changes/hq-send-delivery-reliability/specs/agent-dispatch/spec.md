# agent-dispatch — delta

## ADDED Requirements

### Requirement: Spawn delivery is gated on a ready, stable composer

`gtmux spawn` SHALL NOT deliver a task into a freshly launched agent until the
target pane presents an input-ready, stable composer. Process liveness alone (the
pane's foreground command has left the shell set) SHALL be treated as NECESSARY but
NOT SUFFICIENT: a launched agent whose TUI is still painting a startup banner, a
trust/permission gate, or an unsettled MCP-connect spinner is NOT ready, and pasting
into it risks a truncated goal and a swallowed Enter.

A pane SHALL be considered READY only when its capture shows ALL of: the agent's
input prompt line present (and not a live choice menu), NO startup/trust gate on
screen, NO boot/connect/authentication banner on screen, AND two consecutive
captures byte-identical (settled). The readiness poll SHALL be bounded by the
spawn ready timeout (reusing the existing `spawnReadyTimeout` tune, default 20s) with
backoff. On timeout the system SHALL report `state:"failed"` / `delivered:false`
with the last capture as evidence and SHALL NOT paste — it MUST NOT deliver into a
pane that never became ready. The readiness signatures (banner/gate/prompt-line)
SHALL be per-agent and extensible, NOT hardcoded to one agent.

Only after READY SHALL the existing delivery run: an atomic bracketed paste, a
full-payload (head AND tail) draft confirmation before Enter, and a swallowed-Enter
re-confirm that never blindly re-sends Enter or re-pastes the payload. Readiness
gates that machinery; it does not replace it.

#### Scenario: Delivery waits out a boot banner

- **WHEN** `spawn` launches an agent whose pane shows an "MCP servers need
  authentication" / connecting banner while the composer is not yet stable
- **THEN** the goal is NOT pasted until the banner clears and two consecutive
  captures are identical, so the full goal lands rather than a truncated head

#### Scenario: A pane stuck at a gate to the deadline fails, does not paste

- **WHEN** the target pane is still at a trust/permission gate (or still showing a
  boot banner) when the ready timeout elapses
- **THEN** `spawn` reports `state:"failed"` / `delivered:false` with the capture as
  evidence and pastes nothing — the goal is never sent through the gate keypress

#### Scenario: Process-up alone does not authorize delivery

- **WHEN** the pane's foreground command has become the agent but its composer is
  still mid-boot (banner present or capture still changing)
- **THEN** the pane is NOT yet READY and no paste occurs until the composer is
  input-ready and settled
