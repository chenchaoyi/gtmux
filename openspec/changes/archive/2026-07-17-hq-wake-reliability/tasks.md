# Tasks: hq-wake-reliability

## 1. Phase A — stop the silent drops (P0)

- [x] 1.1 `.sending` orphan reclaim: a drain first renames any `.sending` claim older
      than 60s back to `.txt`. Test: a stranded claim is delivered by the next drain;
      a fresh claim (a live drainer) is left alone.
- [x] 1.2 goal-changed dedup → fingerprint + TTL: `goalchanged/<pane>` holds
      `{hash: sha256(clean prompt), ts}`; suppress only on `hash ==` AND `age < 5m`.
      Move the pane goal to its own `goal/<pane>` marker; `doneGoal` reads that. Tests:
      same prompt inside the window suppressed, same prompt after it wakes again,
      different prompt with the same 40-rune head wakes, legacy plain-text marker
      degrades to one extra wake.
- [x] 1.3 Prompt classification: `transcript.ClassifyUserPrompt(raw) (text, kind)` with
      kinds user/slash/drop; `CleanUserPrompt` keeps its signature over it. Tighten
      `isClaudeMetaPrompt` to the exact wrapper tags; `stripInjected` also drops
      `» gtmux·` lines. Hook wakes a slash command as `goal:"(slash-command) /x"`.
      Tests: slash wakes with the label, harness-only content stays silent, an echoed
      wake line stays silent, a real prompt is unaffected.
- [x] 1.4 New `internal/hqpane`: `Find()` / `FindOther(about) (pane, self)` /
      `SeenRecently()` — the `@gtmux_hq_home` stamp, EvalSymlinks-normalized
      `pane_current_path`, normalized `pane_start_path`; injectable lister for tests.
      Point `hook.findSupervisorPane` + `app.findHQPane` at it. `gtmux hq` stamps
      `@gtmux_hq_home` (VALUE = the home it serves, so a shared tmux server can't
      resolve another install's — or a test's — HQ) on spawn. Tests: symlinked home
      resolves, the stamp resolves after a `cd`, another home's stamp does not,
      self-pane returns self=true (never self-wake) distinctly from "no HQ".
- [x] 1.5 Queue-on-miss: when no HQ resolves but one was stamped within 2h, enqueue
      instead of dropping. Test: wake queued while HQ is missing, delivered on the next
      drain that finds it; nothing queued when no HQ was ever seen.

## 2. Phase B — ack + retry (P0)

- [x] 2.1 Export `dispatch.ContainsHead` (the normalized head matcher) for reuse; keep
      the internal `containsHead` call sites working.
- [x] 2.2 hqnudge delivery splits paste from submit: `tmux.Paste` + `tmux.SendKey(Enter)`
      replace `SendText(…, enter: true)`. io gains `paste`/`enter`/`captureFull`.
- [x] 2.3 Batch id: coalesced line ends ` · #<id>`, `id = sha256(claim names + payload)[:6]`.
      Test: a re-send of the same batch carries the same id; a new batch with identical
      text carries a different one.
- [x] 2.4 Requeue instead of remove: any paste/Enter error, and any unconfirmed ack,
      renames the `.sending` claims back to `.txt`. Remove ONLY after a confirmed ack.
      Tests: paste error requeues, Enter error requeues, unconfirmed ack requeues,
      confirmed ack removes.
- [x] 2.5 Ack read: after Enter, one frame gap then `captureFull` + `ContainsHead(cap, id)`.
      Test: id on screen → delivered; absent → requeued.
- [x] 2.6 Failure counter + `wake-degraded`: `hqnudge` counts consecutive failures
      (reset on a confirmed ack) and exposes it; `hqfeed` gains the
      `gtmux:wake-degraded` control kind; the serve tick raises it once per transition
      (control record at important severity + best-effort HQ line + `notify.Send`),
      deduped by `markerChanged`. Tests: 3rd failure escalates once, a confirm resets,
      recovery does not re-alert.

## 3. Phase C — flapping (the commander's痛点)

- [x] 3.1 `resource`: `diskHysteresisGB` (2) / `loadHysteresis` (0.15) /
      `confirmSamples` (3) / `minRestateMinutes` (30) config with defaults + clamping;
      `MachineTierSticky(prev, m)` (rise on entry thresholds, fall only past the exit
      band). `Snapshot` stays raw. Tests: 15.1→14.9→15.1 GB holds red, 17 GB clears it,
      load 0.95↔1.05 holds amber, 0.8 clears it.
- [x] 3.2 `app/tiergate.go`: pure `tierStep(state, obs, now, confirm, minRestate)` →
      (state, nudge) with the confirmation window, the min restate interval, and the
      escalation bypass; persisted as JSON in the state dir. Tests: 3 samples commit,
      2 do not, restate suppressed inside the window, escalation always nudges.
- [x] 3.3 Wire the resource nudge through `MachineTierSticky` + the gate in
      `slowTickEval`; leave `limits·warn` on `markerChanged`. Test the wiring end to
      end with an injected sample sequence.

## 4. Phase D — latency and flood (P1)

- [x] 4.1 Fast drain ticker: `fastTickInterval` (3s) + `Deps.OnFastTick` in the server
      hub's single goroutine, wired to the `Pending()`-gated drain in app. Test: the hub
      calls OnFastTick on the fast cadence; an empty queue captures nothing.
- [x] 4.2 `hqwake.PriorityOf(line)` — the class→priority table (0 decision-dense /
      1 outcome / 2 standing). Fixture test over every class.
- [x] 4.3 Queue entries carry `.p<prio>` in the filename; drain orders by priority then
      time; legacy names parse as priority 1 / due now. Tests: goal-changed overtakes
      resource·warn; a legacy-named entry still drains.
- [x] 4.4 Caps: 8 lines per batch with a `+N more queued` tail; 200-entry queue evicting
      the lowest-priority oldest. Tests: 12 due → 8 + tail, remainder drains next; a
      full queue evicts a standing warning, not the decision-dense entry.

## 5. Spec ⇄ code ⇄ test ⇄ docs consistency

- [x] 5.1 Bump `hqPlaybookVersion` 2 → 3 and teach the playbook the `#<id>` re-send
      rule, the `(slash-command)` goal payload, and the `wake-degraded` class.
- [x] 5.2 `docs/cli.md`: the `resource` config object gains the four new keys.
- [x] 5.3 CLAUDE.md: the wake-reliability semantics (acked delivery + `#id`, the
      hardened HQ resolution, the flapping damping) in the HQ section.
- [x] 5.4 `make check` green (gofmt + vet + staticcheck + `go test -race`);
      `npx @fission-ai/openspec validate --specs --strict` passes;
      `scripts/check-design.sh` green.
- [x] 5.5 Sync the deltas into `openspec/specs/` and archive this change once merged.
