# agent-dispatch (delta)

## MODIFIED Requirements

### Requirement: Spawn delivery is gated on a ready, stable composer

`gtmux spawn` SHALL NOT deliver a task into a freshly launched agent until the
target pane presents an input-ready, stable composer. Process liveness alone (the
pane's foreground command has left the shell set) SHALL be treated as NECESSARY but
NOT SUFFICIENT: a launched agent whose TUI is still painting a startup banner, a
trust/permission gate, or an unsettled MCP-connect spinner is NOT ready, and pasting
into it risks a truncated goal and a swallowed Enter.

A pane SHALL be considered READY only when its capture shows ALL of: the agent's
input prompt line present (and not a live choice menu), NO startup/trust gate on
screen, NO boot/connect banner on screen, AND two consecutive captures
byte-identical (settled). For an agent whose driver provides a readiness
signal (e.g. a session-start hook event on the target pane after the launch
moment), that signal MAY short-circuit the settle requirement: once the event is
observed, a SINGLE input-ready capture (prompt line present, no gate, no banner)
suffices, because the event deterministically proves the session came up. The
absence of the readiness event SHALL NOT delay or fail the spawn — it only means
the full screen-based gate applies unchanged. The readiness poll SHALL be bounded by
the spawn ready timeout (reusing the existing `spawnReadyTimeout` tune, default 20s)
with backoff. On timeout the system SHALL report `state:"failed"` / `delivered:false`
and SHALL NOT paste — it MUST NOT deliver into a
pane that never became ready. The readiness signatures (banner/gate/prompt-line)
SHALL be per-agent and extensible, NOT hardcoded to one agent.

A boot banner is chrome that RESOLVES BY WAITING. A STANDING NOTICE — a bottom-region
line naming an action only the user can take, such as
`N MCP servers need authentication · run /mcp` — SHALL NOT be treated as boot noise: it
never clears on its own, so treating it as such makes the gate unsatisfiable on any
machine that carries one, timing every spawn out into a false `NOT delivered` while the
session sits there with an empty composer. A settled composer that merely carries a
standing notice SHALL read as READY. A pane that is genuinely still booting — a
connect/load/init banner where the composer will be, or no composer row at all — SHALL
still read as NOT ready.

On timeout the reported evidence SHALL LEAD with the specific condition that kept the
pane not-ready — the matched boot-banner line, the startup gate, a live choice menu, the
absent composer row, an agent that never launched, or a composer that never settled — so
a standing-notice block cannot be misread as a slow agent. The capture SHALL accompany
it as its BOTTOM REGION rather than the full scrollback, so the verdict is not drowned in
the evidence it introduces.

Only after READY SHALL the existing delivery run: an atomic bracketed paste, a
full-payload (head AND tail) draft confirmation before Enter, and a swallowed-Enter
re-confirm that never blindly re-sends Enter or re-pastes the payload. Readiness
gates that machinery; it does not replace it.

#### Scenario: Delivery waits out a boot banner

- **WHEN** `spawn` launches an agent whose pane shows a connecting/loading banner
  while the composer is not yet stable
- **THEN** the goal is NOT pasted until the banner clears and two consecutive
  captures are identical, so the full goal lands rather than a truncated head

#### Scenario: A standing authentication notice does not block delivery

- **WHEN** the target pane shows its composer prompt, has settled across two captures,
  and also carries a persistent `10 MCP servers need authentication · run /mcp` line
- **THEN** the pane is READY and the goal is delivered — the notice is standing chrome,
  not boot noise, and cannot keep the pane not-ready indefinitely

#### Scenario: A pane stuck at a gate to the deadline fails, does not paste

- **WHEN** the target pane is still at a trust/permission gate (or still showing a
  boot banner) when the ready timeout elapses
- **THEN** `spawn` reports `state:"failed"` / `delivered:false` with the capture as
  evidence and pastes nothing — the goal is never sent through the gate keypress

#### Scenario: The timeout evidence identifies the blocker

- **WHEN** the ready gate times out because a bottom line matched a boot-banner
  signature
- **THEN** the reported evidence names that matched line ahead of the capture, not only
  "composer not ready within the ready timeout"

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

### Requirement: Dispatch ledger and needs-you view

The system SHALL record each `gtmux spawn` dispatch (task id → pane → goal → model
→ status), INCLUDING what the dispatch created (its session/window, worktree path,
and branch, for later reclamation), an additive `source` field
(`hq-dispatched` | `user-direct` | `agent-self`), and the DELIVERY VERDICT
(`delivered` plus an additive `state`), and expose `gtmux tasks [--json]`.
`gtmux spawn` SHALL stamp `source: "hq-dispatched"`; `user-direct`/`agent-self`
entries are ones HQ back-fills from work it sensed (gtmux does not fabricate them).

A ledger entry's lifecycle status (working → waiting → done) SHALL be derived from the
dispatched pane's live radar state ONLY for a dispatch whose goal actually reached the
agent. An entry the ledger records as never delivered SHALL read `undelivered` instead,
whatever the pane is doing — a ready-timeout or a refused delivery leaves a live, empty,
IDLE agent pane, and deriving from that pane alone renders a dispatch that never ran as
`done`, with its recorded goal (the INTENT, not the result) beside it. A `queued`
delivery is not undelivered: the agent accepted it behind the current turn. A delivery
that lands LATER by another channel (a `gtmux send` rescue carrying that goal into that
pane) SHALL update the entry, so the honest workaround does not leave a permanently wrong
row. The update SHALL be gated on the delivered text being that goal — an unrelated
message typed into the same pane must not launder a dispatch that never happened. `undelivered`
SHALL sort ahead of the needs-you entries — nothing is running at all.

`gtmux tasks` SHALL lead with entries needing attention (a tracked pane that is
undelivered, waiting, or done-after-work), the same needs-you-first ordering the digest
uses. Every surface presenting this status SHALL respect it: the digest SHALL NOT file an
undelivered dispatch under its completed section (by pane status alone it would, which is
the same false signal on the surface the supervisor actually reads). The `source` and `state` fields are additive and optional — an entry without
`source` is treated as `hq-dispatched`, and an entry without `state` is judged by
`delivered` alone.

#### Scenario: A dispatch is tracked

- **WHEN** `gtmux spawn <goal>` succeeds
- **THEN** a ledger entry exists for it with `source: "hq-dispatched"` and
  `gtmux tasks` lists it with its live status

#### Scenario: A never-delivered dispatch is not done

- **WHEN** a spawn times out at the ready gate, leaving a live agent pane the radar
  reports as idle, and `gtmux tasks` runs
- **THEN** the entry reads `undelivered`, never `done`, and is listed ahead of the
  waiting entries

#### Scenario: The digest does not file it as completed

- **WHEN** the digest renders a pane whose dispatch is undelivered and whose radar status
  is idle
- **THEN** the row is grouped with what needs you, not with completed work, and its badge
  says `undelivered`

#### Scenario: A queued delivery still derives from the pane

- **WHEN** a dispatch was accepted behind the current turn (`state:"queued"`) and its
  pane is working
- **THEN** the entry reads `working` — queued is accepted, not undelivered

#### Scenario: A rescue send closes the record

- **WHEN** a `gtmux send` lands the goal in a pane whose ledger entry is undelivered
- **THEN** the entry is recorded as delivered and its status derives from the pane again

#### Scenario: Needs-you ordering

- **WHEN** `gtmux tasks` runs and a tracked pane is waiting or done-after-work
- **THEN** that entry is listed ahead of still-working ones

#### Scenario: Source round-trips

- **WHEN** a ledger entry is written with a `source` and read back
- **THEN** the same source is returned; a legacy entry without one reads as
  `hq-dispatched`
