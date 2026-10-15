# Codex Session Path Scan Index

## Why

`GatherAgents()` repeatedly traversed the Codex session tree while resolving different
facts for the same sessions. On the observed machine, a single `agents --json` call took
several seconds with 20 active Codex sessions.

## What changes

Use a lazy, gather-owned Codex path index so one gather enumerates candidate rollout
paths once and reuses resolved paths for its transcript reads. Continue reading mutable
transcript contents on demand. Preserve existing session matching, ordering, radar output,
and non-Codex behavior.

## Impact

Internal implementation change to `agent-radar`, Codex transcript lookup, and Codex
binding recovery. No CLI/API schema or user-visible behavior changes. Detailed design,
compatibility boundaries, and performance evidence follow below. Test cases are in
[`test-spec.md`](./test-spec.md); measured results are in
[`docs/test/CODEX-SESSION-PATH-SCAN-2026-10-15.md`](../../../docs/test/CODEX-SESSION-PATH-SCAN-2026-10-15.md).

## Surfaces

- **终端 / terminal:** no output or command changes.
- **菜单栏 / menubar:** no display or interaction changes.
- **手机 / phone:** no display or interaction changes.
- **iPad:** no display or interaction changes.
- **Web:** no display or interaction changes.

## Outcome

Reduce repeated Codex sessions-tree traversal during one `GatherAgents()` call while preserving current Codex session/path matching and all user-visible radar output. The optimization is limited to Codex; Claude lookup, refresh cadence, native retention, CLI/API schemas, and transcript state interpretation remain unchanged.

- Base: `origin/main` at `18b54e5c13248a594121b238bf5ac1b367acd7f9`
- Branch: `perf/codex-session-scan-index`
- Test specification: [test-spec.md](./test-spec.md)

## Evidence and Current Mechanism

- `internal/transcript/codex.go:85-120` performs four `filepath.Glob` searches for every `codexLogPaths(sessionID)` call, then validates metadata, sorts by basename, and removes archive/live duplicates.
- `internal/transcript/transcript.go:97-105`, `internal/transcript/codex.go:221-230`, `internal/transcript/codexidentity.go:38-54`, and `internal/transcript/sessionbinding.go:29-69` resolve the same Codex session paths for independent facts.
- One radar gather starts at `internal/radar/agents.go:1006-1019`; tmux Codex reconciliation calls the boundary reader at `internal/radar/agents.go:1154-1203`, while native rows request last-message time, status, identity, policy eligibility, errors, and outcome at `internal/radar/agents.go:1517-1575` and `internal/radar/agents.go:1597-1666`.
- Pending binding recovery adds another identity/path lookup through `internal/resume/codexbinding.go:183-205`; policy eligibility resolves identity again at `internal/sessionpolicy/policy.go:43-46`.
- Existing correctness coverage already pins continuation handling at `internal/transcript/transcript_test.go:82-153` and Codex radar state transitions at `internal/radar/gather_test.go:354-438` and `internal/radar/nativerow_test.go:126-206`.

## Scope

### In scope

- A lazy, gather-owned Codex path index/resolver. Its lazy initialization and result memoization must be synchronized if any helper can access it concurrently.
- Path-aware internal transcript readers used by the radar gather.
- Narrow dependency threading through radar helpers and Codex binding/policy checks needed by that gather.
- Regression tests for identity, continuation/archive semantics, sorting/deduplication, freshness boundary, and concurrent gathers.
- Structural performance proof that a gather discovers the Codex tree at most once.

### Out of scope

- Claude or other agent path lookup changes.
- Process-wide or cross-refresh transcript caches.
- Caching parsed status, last message, error, turn boundary, policy, or other mutable content.
- Changes to `agents --json`, HTTP payloads, refresh intervals, native-record expiry, or menu behavior.
- A redesign of transcript loading outside the radar/gather request path.

## Design considerations

### Principles

1. Preserve the existing `codexLogPaths` matching contract exactly before optimizing traversal.
2. Cache only path discovery within one gather; read mutable transcript facts from the files when requested.
3. Keep each concurrent gather isolated with no package-global mutable resolver state.
4. Leave non-Codex resolution on its current code path.

### Decision drivers

1. Replace repeated filesystem tree traversal with one candidate enumeration per gather, then use before/after measurements to determine whether this meets the observed workload.
2. Retain compatibility with current and legacy metadata, resumed rollouts, archived files, ordering, and deduplication.
3. Keep invalidation simple: a new gather creates a new filesystem snapshot.

### Options considered

#### Option A — gather-scoped lazy candidate index (recommended)

Create one resolver per `gatherAgents()` invocation. On the first Codex lookup it enumerates live and archived candidate filenames once, preserving live-before-archive precedence. Per-session resolution filters that in-memory candidate list with the same filename rules and metadata checks as `codexLogPaths`, then memoizes only the resulting path slice for the remainder of the gather. This still inspects inactive rollout filenames once per Codex-using gather; the targeted reduction is from repeated tree traversals (and four globs per lookup) to one candidate enumeration, not to zero inactive-file work. Record a fresh baseline and compare it with the implementation before deciding whether this reduction is sufficient.

Pros:

- One candidate filename enumeration per gather, independent of active Codex session count.
- No cross-gather invalidation, stale shared state, or background watcher; synchronize within a resolver only if gather helpers can call it concurrently.
- Can keep the existing public transcript functions as fresh, direct wrappers for non-radar callers.

Cons:

- Requires threading a scoped reader/resolver through radar helpers and binding recovery used by radar.
- A rollout created after the scope snapshot is visible on the next gather, not halfway through the current gather.

#### Option B — per-session memoization inside one gather

Cache `sessionID -> []path` after the first existing `codexLogPaths(sessionID)` call.

Pros:

- Smallest source diff and almost no matching-logic refactor.
- Removes duplicate scans for the same session.

Cons:

- Still traverses the complete date tree once for every active Codex session; with many sessions it leaves the dominant scaling problem in place.
- Does not meet the desired “one discovery per gather” structural target.

#### Option C — process-wide cache with filesystem invalidation

Share a Codex path index across gathers and invalidate it using directory metadata or filesystem watching.

Pros:

- Lowest steady-state directory traversal cost.

Cons:

- Correct invalidation spans newly created date directories, archive moves, atomic replacements, and configured `CODEX_HOME` changes.
- Adds synchronization and stale-read failure modes to a correctness-sensitive status path.
- Conflicts with the requirement to avoid cross-filesystem-change caching for this fix.

### Decision

Use Option A. Option B is safe but likely under-delivers on machines with many active sessions. Option C has disproportionate invalidation and concurrency risk.

## Implementation Plan

### 1. Extract the exact Codex path-matching contract behind a resolver

Touchpoints: `internal/transcript/codex.go:69-120`, `internal/transcript/transcript_test.go:82-153`.

- Introduce a private path-resolver abstraction and a gather-scoped Codex index constructor in `internal/transcript`.
- Keep `codexLogPaths(sessionID)` as the direct/fresh compatibility entry point for existing non-scoped callers.
- Have both direct and scoped resolution share one matcher that preserves:
  - rejection of empty or glob-meta-containing session IDs (`*/?[]\\`);
  - current `session_meta.id` and legacy `session_meta.session_id` identity;
  - exact base rollout plus `_*.jsonl` continuation matching;
  - rejection of a continuation whose metadata is missing or names another session;
  - the current legacy acceptance of an unsuffixed direct-name rollout with missing metadata;
  - live-tree discovery before `archived_sessions`;
  - stable basename sorting and basename deduplication so the live copy wins.
- Copy returned path slices, or otherwise prevent callers from mutating the index’s stored result.

Acceptance:

- Existing direct APIs return the same paths in the same order for all current fixtures.
- The scoped resolver discovers the candidate tree no more than once per instance.
- No Claude function or path pattern is edited.

### 2. Add a Codex-only scoped transcript fact reader

Touchpoints: `internal/transcript/transcript.go:97-109`, `internal/transcript/codex.go:221-230`, `internal/transcript/codexidentity.go:38-54`, `internal/transcript/sessionbinding.go:27-69`.

- Refactor only the Codex readers reachable from `gatherAgents()` into shared implementations that accept the path resolver; retain current exported functions as wrappers using direct/fresh resolution.
- Expose the smallest scoped surface needed by radar: last message time, last turn boundary, identity/client, and binding prompts.
- Keep `LastMessageError` outside the scoped reader. Its non-Claude behavior is contractually always false; make the Codex gather path preserve that result without invoking another Codex path scan.
- Do not refactor `FirstMessageTime`, general transcript loading, or `LogRevision` unless implementation proves one is reachable from this gather path.
- Cache paths only. Each scoped fact read must reopen/stat/read the current file contents so an append that occurs during the gather can still update the fact for an already-discovered path.
- Do not place Claude behind the new resolver; generic agent methods must delegate non-Codex agents to their unchanged functions.

Acceptance:

- No mutable transcript fact is stored in the scoped index.
- Appending to an already indexed rollout is observed by a later fact read in the same scope.
- A newly created rollout is observed by a newly created gather scope.

### 3. Thread one scope through the complete radar gather

Touchpoints: `internal/radar/agents.go:1006-1019`, `internal/radar/agents.go:1154-1203`, `internal/radar/agents.go:1261-1290`, `internal/radar/agents.go:1345-1347`, `internal/radar/agents.go:1446-1578`, `internal/radar/agents.go:1597-1666`, `internal/resume/codexbinding.go:182-216`, `internal/sessionpolicy/policy.go:43-46`.

- Create the scoped reader inside `gatherAgents()`, not at package initialization and not in `GatherAgents()` global state.
- Pass it explicitly to tmux reconciliation and `nativePanes`, including `NativeStatus`, turn outcome, native client identity, and last-message reads. For Codex `LastMessageError`, preserve its constant `(false, "")` result without resolving paths.
- Include both `resume.CodexBindingForSession` and `resume.CompleteCodexBinding`, reached from `nativePanes`, in the scoped path. Add narrow scoped variants or callbacks for binding recovery. For policy eligibility, resolve Codex identity once through the scope and pass the result into existing policy lookup rather than adding a second resolver surface. Preserve current public functions for callers outside gather.
- Compute repeated per-row facts once when practical (for example verified identity/eligibility) and reuse the value while assembling that row.
- Keep sorting, deduplication, emitted fields, and marker mutation order unchanged.

Acceptance:

- One `GatherAgents()` call uses exactly one resolver instance across tmux and native Codex rows.
- Two simultaneous gathers use different resolver instances and pass `go test -race`. Keep each resolver gather-owned; synchronize lazy initialization and maps if helpers can access them concurrently, and return copies of cached path slices.
- A gather with no Codex transcript lookup does not eagerly scan the Codex tree.
- Claude rows continue through the existing Claude lookup path.

### 4. Lock compatibility and traversal-count behavior with tests

Implement the cases in the companion test specification, primarily in `internal/transcript/transcript_test.go` and `internal/radar` tests. Add a narrow discovery seam/counter rather than timing assertions.

Acceptance:

- Tests prove current/legacy metadata, continuation rejection/acceptance, archive precedence, basename ordering/deduplication, in-place append freshness, next-gather discovery, and concurrent-scope isolation.
- A radar test proves multiple Codex fact reads and multiple Codex sessions cause one candidate-tree discovery for that gather.
- Existing Codex status regressions remain green without changing expected JSON/status values.

### 5. Validate behavior, performance shape, and process guard before review handoff

- Run targeted tests first:
  - `go test -race ./internal/transcript`
  - `go test -race ./internal/radar ./internal/resume ./internal/sessionpolicy`
- Run repository Go gates:
  - `gofmt -l` on changed Go files and fail on output.
  - `go vet ./...`
  - `make lint`
  - `go test -race ./...`
- Build the CLI and compare representative `gtmux agents --json` output fields/order against the baseline fixture or a captured normalized snapshot.
- Before editing, run the same local `agents --json` workload against this branch's base and record at least five samples, median/p95 elapsed time, CPU time, active Codex session count, and maximum gtmux process count. Repeat after implementation with the same `CODEX_HOME` and process monitor. The monitor must stop launching validation work at count 9, leaving one-process headroom, and terminate if count reaches 10. Define process count as all live processes whose executable or command identifies this gtmux checkout or app bundle.
- Review `git diff --stat`, `git diff`, and `git status`; confirm no Claude lookup file changed and no product files outside the approved scope changed unexpectedly.

Stop condition:

- All targeted and full gates pass; structural scan-count tests show one discovery per Codex-using gather; behavior comparisons are unchanged; the measured reduction is sufficient for the observed workload; monitored process count stays at or below 10. Then commit the optimization branch for review. PR submission is not authorized for this branch by the current request.

## Acceptance Criteria

1. Codex candidate discovery occurs at most once within one `GatherAgents()` invocation, even with multiple sessions and repeated fact reads.
2. Each new gather uses a fresh index; there is no process-wide path or status cache.
3. File-content facts are not cached: appends to discovered files remain visible within the scope.
4. Both `session_meta.id` and legacy `session_meta.session_id` resolve exactly as before.
5. Base and continuation rollouts combine only when metadata identity matches; mismatched/missing continuation metadata is rejected.
6. Live and archived copies keep current live-first, stable-basename sort, and basename-dedup behavior.
7. Invalid session IDs remain rejected before filesystem matching.
8. Claude lookup implementation and behavior are unchanged.
9. `agents --json` schema, row ordering, status, error, end reason, client, eligibility, and adoptability are unchanged for equivalent filesystem input.
10. Concurrent gathers are race-free and share no mutable resolver/index.
11. Local validation never allows the gtmux process count to exceed 10.

## Risks and Mitigations

- **Semantic drift while replacing glob-per-session matching.** Keep one shared matcher and encode every existing edge case in table-driven tests before switching radar to the scoped resolver.
- **Snapshot misses a rollout created mid-gather.** Limit the snapshot lifetime to one gather; the next normal poll discovers it. Continue reading current contents from already discovered paths.
- **Hidden direct lookup escapes the scope.** Add a discovery-count integration test across tmux, native, binding, and eligibility paths; inspect all Codex transcript calls in `internal/radar`, `internal/resume`, and `internal/sessionpolicy`.
- **Concurrent gather races.** Keep per-instance candidate/path maps private to the owning gather, avoid package-global resolver installation, and run race tests with concurrent scopes/gathers. Synchronize lazy initialization and map access if helper calls can overlap.
- **Path slice mutation corrupts later reads.** Return copies or immutable views from the resolver.
- **Optimization shifts cost to reading every metadata header.** Build the index from candidate filenames and lazily validate metadata only for requested sessions; do not parse full transcripts during discovery.
- **Over-broad refactor affects Claude.** Keep the new type and variants Codex-specific and verify the diff contains no changes to `internal/transcript/claude.go`.

## ADR

### Decision

Use a lazy, gather-owned Codex candidate-path index and explicitly thread its scoped transcript reader through radar’s Codex fact consumers.

### Drivers

- Remove repeated date-tree traversal across sessions and facts.
- Preserve all existing Codex filename/metadata compatibility behavior.
- Avoid invalidation and synchronization complexity.

### Alternatives considered

- Per-session gather memoization: rejected as the primary solution because it still scans the complete tree once per active session.
- Process-wide invalidated cache: rejected because filesystem invalidation and concurrency risks exceed this fix’s scope.

### Why chosen

It provides a structural one-discovery-per-gather bound while keeping freshness and concurrency semantics easy to reason about. Its lifetime matches the caller that causes the repeated lookups.

### Consequences

- Radar gains an explicit scoped dependency through several private helpers.
- New rollout files appear on the next gather, while appends to known rollouts remain visible immediately to later reads.
- Existing non-radar transcript APIs keep their direct fresh lookup semantics.

### Follow-ups

- After merge, compare local `agents --json` latency and server CPU with the recorded baseline; do not add a longer-lived cache unless evidence still requires it.
- If transcript content reads become the next bottleneck, plan that separately with file revision based invalidation; it is not part of this change.
