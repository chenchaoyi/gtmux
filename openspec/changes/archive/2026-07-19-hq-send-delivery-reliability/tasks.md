# Tasks: hq-send-delivery-reliability

Each task is a pure increment behind its own PR; land it green (`make check`) before
the next. A/B are independent and can proceed in parallel.

## A. Spawn delivery: launched → ready → content-verified → submitted

### A1 — Screen-based readiness probe (the new `ready` state)

- [x] A1.1 Add a per-agent `bootBanners` table in `internal/prompt/prompt.go`, keyed
  like `startupGates` (`""` = Claude default): `MCP servers need authentication`,
  `Connecting`, `Starting`, `Loading`, the auth/spinner lines. Extensible per agent.
- [x] A1.2 Add `hasBootBanner(capture, agent) bool` (matches `bootBanners[""]` +
  `bootBanners[agent]`) and `hasPromptLine(capture, agent) bool` (the input glyph row
  is present in the bottom region, and is NOT a live `WaitingOptions` menu).
- [x] A1.3 Add `IsComposerReady(capture, agent) bool :=
  hasPromptLine ∧ !IsStartupGate ∧ !hasBootBanner`. Pure string predicate.
- [x] A1.4 Unit tests in `prompt_test.go`: a boot-banner capture is NOT ready; a
  trust-gate capture is NOT ready; a live menu is NOT ready (a menu ≠ a goal-ready
  composer); a clean `❯` composer IS ready; per-agent banner phrase hits.
- [x] A1.5 In `internal/dispatchbridge/dispatchbridge.go`, strengthen
  `WaitAgentReady`: keep the `pane_current_command` liveness check to reach
  `launched`, then poll `tmux.CaptureFull` until `prompt.IsComposerReady` is true for
  TWO consecutive byte-identical captures (stable), or the deadline. Backoff between
  polls. Return false on timeout (no paste).
- [x] A1.6 Test the two-stable-sample gate (a still-changing capture is not "ready"
  until it settles; a settled ready capture returns true within the budget).

### A2 — Atomic paste + content-verify retry (reuse, wire-through)

- [x] A2.1 Confirm `spawn.go`'s delivery path (`dispatch.Deliver` via
  `dispatchbridge.DispatchIO`/`DeliverOpts`) runs ONLY after the strengthened
  `WaitAgentReady` returns true — no paste before `ready`. (Call site already ordered
  this way; assert it and add a comment tying it to the handshake.)
- [x] A2.2 Regression test: a spawn whose pane shows a boot banner for the first N
  polls then settles delivers the FULL goal (head+tail present), not a truncated
  head — i.e. the readiness gate prevents the mid-boot paste. (Use the injected
  `dispatch.IO` / fake capture, no real tmux.)

### A3 — Enter confirmation (reuse, document the state)

- [x] A3.1 No code change to `Deliver`'s swallowed-Enter re-confirm (already correct
  per `send-submit-reliability`). Add the comment in `spawn.go`/`dispatchbridge.go`
  naming the four states so the handshake is legible end to end.
- [x] A3.2 Confirm (test or assertion) that on a readiness timeout spawn reports
  `state:"failed"`, `delivered:false`, with the last capture as evidence — it never
  pastes into a not-ready pane.

### A4 — Startup-gate pre-clear

- [x] A4.1 Fold the gate/banner into readiness (done via A1.3): a gate or banner on
  screen ⇒ not ready ⇒ no delivery. Test that a spawn stuck at a trust gate to the
  deadline fails with evidence rather than pasting through the gate.
- [x] A4.2 Assert spawn's existing cwd-trust discipline in a comment/spec so a
  spawned session does not hit an avoidable trust gate in the first place (no new
  code if already ensured; otherwise a minimal pre-trust step).

## B. `waiting → non-waiting` emits an acked `resolved`

### B1 — Transition detector in the single writer

- [x] B1.1 Add a `resolvedTransitionSweep` in `internal/hq/slowtick.go` (sibling to
  `stuckDispatchSweep`, single writer). Per sampled pane, compare the last-seen
  waiting kind (`hqwake/resolved-last-<pane>` marker) against the current
  `waiting/<pane>` marker; on `wasWaiting != "" && nowWaiting == ""`, emit `resolved`
  for `wasWaiting` (unless the hook already announced it — see B1.3). Update the
  marker to the current state each pass.
- [x] B1.2 Emit through the SAME `nudgeResolved`/`hqnudge` path the hook uses (not a
  raw `SendText`) so it inherits ack/retry/dedup/degradation.
- [x] B1.3 Optional conservative screen fallback: a pane still carrying the `waiting`
  marker whose capture has visibly advanced past the gate (no `WaitingOptions`, no
  `IsStartupGate`, active turn) MAY be treated as cleared. Keep marker-disappearance
  as the primary signal.
- [x] B1.4 Tests in `slowtick_test.go`: a pane that goes waiting→clear with no
  resolving hook event fires exactly one `resolved`; a pane still waiting fires none;
  the marker tracks state across ticks.

### B2 — `resolved` dedup + acked delivery

- [x] B2.1 Add the shared dedup marker: `nudgeResolved` (hook fast path) stamps
  `resolved-emit-<pane>` (cleared kind + short TTL) when it emits; the slow-tick sweep
  checks it (`recentlyResolvedByHook`) and skips a duplicate. Exactly one `resolved`
  per clear, whichever channel sees it first.
- [x] B2.2 Test the dedup both ways: (a) hook emits first → slow-tick is silent;
  (b) no hook event → slow-tick is the sole emitter.
- [x] B2.3 Confirm (test/assertion) `resolved` rides the acked `hqnudge` channel end
  to end — a delivery failure retries and, on repeated failure, escalates via the
  existing `wake-degraded` path. No bespoke best-effort send.

## C. Spec / docs / gate (each PR keeps this truthful)

- [x] C.1 `make check` green (gofmt + vet + staticcheck + `go test -race`) on every
  PR; `CGO_ENABLED=0 go build ./cmd/gtmux` stays clean.
- [x] C.2 Sync the deltas into `openspec/specs/agent-dispatch/spec.md` and
  `openspec/specs/hq-wake-protocol/spec.md`; `openspec validate
  hq-send-delivery-reliability --strict` and `openspec validate --specs --strict`
  pass.
- [x] C.3 No wire/CLI-surface change → `docs/cli.md`, `api/contract.md`,
  `internal/app/help.go`, and the CLAUDE.md command list are unaffected (grep to be
  sure). If the readiness timeout gains a documented config knob beyond the existing
  `spawnReadyTimeout`, document it.
- [x] C.4 Archive the change once A + B are merged (sync-specs + archive-change),
  keeping these checkboxes truthful.
