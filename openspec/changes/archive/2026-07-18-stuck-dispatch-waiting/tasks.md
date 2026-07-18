# Tasks: stuck-dispatch-waiting

## 1. Detection helpers (prompt)

- [x] 1.1 `prompt.IsStartupGate(capture, agent)` — scoped to the trust/PERMISSION startup
      gate (NOT the resume/theme picker). Per-agent gate phrases (default = Claude's),
      overridable via the agent profile.
- [x] 1.2 Tests: trust gate → true; resume picker → false (keep the existing exclusion);
      a normal turn/idle screen → false.

## 2. Radar guard (display) — never `done` for a stuck pane

- [x] 2.1 In `gatherAgents`, capture ONCE for an idle/running candidate and pass the
      string to a guard: reclassify to `waiting` (kind `startup`/`draft`) when the
      capture is a startup gate, OR a STRUCTURED non-empty `DraftOf` on a TRACKED pane
      (`dispatch.TaskForPane`). Pure — no marker write here.
- [x] 2.2 `taskStatusFor`/digest inherit the fix (they read `p.status`); add a taskscmd
      test proving a stuck pane → `waiting`, not `done`.
- [~] 2.3 (pure decision pinned by IsStartupGate + taskStatus tests; the
      tmux-capture radar path is verified by build + the 5.3 dogfood.) Radar cases — trust gate → waiting;
      tracked-pane draft → waiting; empty idle box / untracked draft → still idle.

## 3. Wake + marker (slow-tick, single writer)

- [x] 3.1 In the serve slow-tick, for a TRACKED pane detected stuck (gate/draft) write
      the `waiting` marker (kind-tagged) so the watchdog escalates + a `waiting` wake
      fires. Deduped; never from the read path.
- [~] 3.2 (slow-tick is tmux-integration; verified by build + dogfood.) Test the slow-tick evaluation writes the marker + would emit `waiting` once.

## 4. Done-wake guard (defense in depth)

- [x] 4.1 `wakeDone` (or its `Stop` call site) skips the `done` wake when the post-Stop
      capture is a startup gate or a draft still holding the payload.
- [~] 4.2 (wakeDone guard is tmux-integration; verified by build + dogfood.) Test: a `Stop` on a startup-gate/draft pane fires no `done` wake.

## 5. Consistency + verification

- [x] 5.1 Fold spec deltas (agent-radar, agent-dispatch, hq-wake-protocol); openspec
      `--strict` green; archive change.
- [x] 5.2 `make check` + `CGO_ENABLED=0 go build ./cmd/gtmux` + `check-design.sh` green.
- [ ] 5.3 Dogfood: spawn a worker into a fresh dir so it hits the trust gate → `gtmux
      tasks` shows `waiting` (not `done`), HQ gets a `waiting` wake; answer the gate →
      it proceeds and completes normally.
