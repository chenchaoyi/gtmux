# agent-dispatch (delta)

## MODIFIED Requirements

### Requirement: Layered verification — deterministic evidence before screen-reading

Delivery verification SHALL be layered to minimize misjudgment. For a hook-equipped
agent (one whose session-events stream records prompt submissions — e.g. Claude
Code), the system SHALL treat the stream as authoritative: a `UserPromptSubmit`
event on the pane whose recorded content head matches the delivered text CONFIRMS the
landing, and no screen read is required. The head recorded by the hook and the head
the verifier matches against SHALL be produced by the SAME normalization pipeline
(one shared implementation), so a cleaning/normalization difference can never make
the verifier ignore a genuine submit event. Screen-reading SHALL be used only as a
FALLBACK for agents that emit no such event (or when the event does not arrive within
a short grace). The fallback SHALL be hardened: it SHALL capture the full screen with
scrollback margin (never a tail sample), locate the input region STRUCTURALLY (by its
separator/box line, so "❯ text" is unambiguously draft vs submitted), find evidence by
PATTERN SEARCH rather than a fixed line offset, and require TWO consecutive consistent
frames before declaring a delivery not-landed (a single frame has misread a transient
context-usage figure and an in-progress compaction bar).

Arbitration between the layers SHALL be positive-monotonic: a stream-confirmed
landing is FINAL and SHALL NOT be overturned by any screen read; before declaring
`delivered:false` the system SHALL perform a final re-read of the event stream over
the delivery window, so a confirmation that arrived after the last poll is never lost
to the timeout. The absence of a stream event SHALL NOT itself constitute failure —
it only defers the judgment to the screen fallback. The reported result SHALL state
which layer judged it (`judged_by: driver | screen`) alongside the existing evidence,
so a misjudgment can be attributed to its layer.

#### Scenario: A hook-equipped agent confirms from the stream

- **WHEN** a task is delivered to a Claude Code pane and a `UserPromptSubmit` event
  with a matching content head appears on the stream
- **THEN** the delivery is confirmed landed WITHOUT reading the screen

#### Scenario: A single transient frame is not a verdict

- **WHEN** the fallback screen-read sees one frame that would read as "not landed"
  (e.g. a transient context-usage figure or a compaction progress bar)
- **THEN** no failure is declared until a second capture agrees

#### Scenario: The stream verdict cannot be overturned by the screen

- **WHEN** the event stream has confirmed a landing and a subsequent screen read
  fails to locate the text
- **THEN** the delivery remains landed — the screen read cannot downgrade a
  stream-confirmed success

#### Scenario: The timeout re-checks the stream before failing

- **WHEN** the verification timeout is reached and the event stream, re-read at
  that moment, contains a matching submit event
- **THEN** the delivery is reported landed (`judged_by: driver`), not failed

#### Scenario: Both sides of the head match use one normalization

- **WHEN** a payload contains content the hook-side cleaning would rewrite (e.g. a
  wrapped or prefixed form)
- **THEN** the verifier's needle is produced by the same pipeline as the hook's
  recorded head, so the match succeeds whenever the event genuinely records the
  delivered payload

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
captures byte-identical (settled). For an agent whose driver provides a readiness
signal (e.g. a session-start hook event on the target pane after the launch
moment), that signal MAY short-circuit the settle requirement: once the event is
observed, a SINGLE input-ready capture (prompt line present, no gate, no banner)
suffices, because the event deterministically proves the session came up. The
absence of the readiness event SHALL NOT delay or fail the spawn — it only means
the full screen-based gate applies unchanged. The readiness poll SHALL be bounded by
the spawn ready timeout (reusing the existing `spawnReadyTimeout` tune, default 20s)
with backoff. On timeout the system SHALL report `state:"failed"` / `delivered:false`
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

#### Scenario: A session-start event short-circuits the settle wait

- **WHEN** the launched agent's driver observes the session-start event for the
  target pane and one capture then shows an input-ready composer
- **THEN** the pane is READY without waiting for two byte-identical captures, so a
  slow-settling boot (e.g. MCP noise churning the screen) does not run out the clock

#### Scenario: A missing session-start event changes nothing

- **WHEN** the launched agent emits no session-start event (older hook set, or the
  capability is disabled)
- **THEN** the screen-based gate applies exactly as specified above, and the missing
  event is never treated as a failure

## ADDED Requirements

### Requirement: One-shot headless dispatch through a driver

`gtmux spawn --oneshot <goal>` SHALL dispatch a one-shot, non-interactive worker
through the agent driver's headless mode (e.g. `claude -p --output-format
stream-json`, `codex exec --json`), and SHALL be accepted ONLY for an agent whose
driver provides the headless capability — otherwise it SHALL refuse with a clear
message rather than silently degrade to an interactive spawn. The worker SHALL still
run inside a tmux pane (its structured output visible, its radar row present, reap
applicable), preserving observability; the non-interactive nature (no takeover) is
the flag's explicit, documented contract. The worker's lifecycle truth (done /
crash) SHALL come from its structured output stream and exit code, not from screen
classification, and its digest row SHALL carry the driver-grade perception tier. The
launch SHALL scrub environment variables that would recursively trigger gtmux hooks
inside the one-shot run. Ledger, tasks, and reap semantics SHALL be unchanged.

#### Scenario: A one-shot worker completes

- **WHEN** `gtmux spawn --oneshot <goal>` runs for a headless-capable agent and the
  worker's output stream reports a result and the process exits zero
- **THEN** the dispatch is recorded done from the stream/exit code — no screen
  classification is involved — and the pane remains inspectable until reaped

#### Scenario: A non-capable agent is refused

- **WHEN** `gtmux spawn --oneshot` targets an agent whose driver has no headless
  capability
- **THEN** the spawn is refused with a message naming the limitation, and no
  interactive session is silently created instead
