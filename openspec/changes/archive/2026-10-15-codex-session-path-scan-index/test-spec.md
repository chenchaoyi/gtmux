# Test Specification: Codex Session Path Scan Index

## Purpose

Prove that the Codex-only gather-scoped index removes repeated filesystem discovery without changing session identity, rollout composition, transcript freshness, radar status, or non-Codex behavior.

## Unit tests: path resolver

Primary location: `internal/transcript/transcript_test.go` alongside the existing continuation coverage at lines 82-153. A dedicated `codex_paths_test.go` is acceptable if it makes the matrix clearer.

### Identity and filename matrix

Create live and archived fixtures and assert the direct resolver and scoped resolver return identical ordered basenames.

| Case | Expected result |
| --- | --- |
| Current `session_meta.id` equals requested ID | Include base rollout |
| Legacy `session_meta.session_id` equals requested ID | Include base rollout |
| Metadata ID differs from requested ID | Reject |
| Unsuffixed direct-name rollout has no readable metadata ID | Preserve current inclusion behavior |
| Suffixed continuation metadata ID equals requested ID | Include |
| Suffixed continuation metadata ID differs | Reject |
| Suffixed continuation has no metadata ID | Reject |
| Empty ID or ID containing `* / ? [ ] \\` | Return empty without discovery |

### Ordering and deduplication

- Put base and multiple continuation rollouts across different live date directories; assert stable order by basename.
- Put the same basename in live and `archived_sessions`; assert one path remains and it is the live path.
- Put an archive-only rollout; assert it remains visible.
- Request an unknown valid ID; assert an empty result without corrupting later known-ID lookups.

### Scope and mutation safety

- Call `Paths(id)` repeatedly and mutate a returned slice; assert later calls still return the intact stored order.
- Append a newer boundary/message to an already indexed path; scoped boundary and message readers must observe it because content is not cached.
- Create a new continuation after the scope has discovered candidates; the current scope may retain its gather snapshot, but a new scope must include the file.
- Change `CODEX_HOME` between scopes; each new scope must use the current configured home. No scope may be reused globally.

### Discovery count

Provide a test-only injectable candidate-discovery function or counter.

- Invalid IDs: zero discoveries.
- First valid lookup: one discovery.
- Repeated facts for the same ID: still one.
- Facts for additional IDs in the same scope: still one.
- A second scope: its own one discovery.

Do not assert wall-clock duration in unit tests.

## Unit tests: scoped fact readers

Primary locations: `internal/transcript/transcript_test.go`, `internal/transcript/sessionbinding_test.go`, and `internal/transcript/codex_client_test.go`.

- Direct and scoped `LastMessageTime` return the same latest timestamp across base plus continuation.
- Direct and scoped last turn boundary select the latest `task_started`, `task_complete`, or `turn_aborted` across rollouts.
- Scoped identity/client accepts both metadata ID spellings and rejects filename/metadata mismatch.
- Scoped binding prompt lookup combines accepted rollouts and ignores a mismatched continuation.
- Codex `LastMessageError` remains `(false, "")` without triggering candidate discovery in the gather path.
- Existing public direct functions remain fresh when called outside a gather.

## Radar integration tests

Primary locations: `internal/radar/gather_test.go` and `internal/radar/nativerow_test.go`.

### Behavior preservation

- Existing “completion clears misrouted wait” behavior remains idle and clears the same markers (`gather_test.go:354-393`).
- Existing idle repaint/new task behavior remains idle then working (`gather_test.go:395-438`).
- Native working state still reconciles a newer complete/abort boundary (`nativerow_test.go:126-206`).
- Resume continuation still supplies completion and last-message time.
- Native client, follow eligibility, `can_read_conversation`, end reason, error, and adoptability fields remain unchanged.
- Equivalent fixture input produces the same row count, row ordering, status, and JSON fields before and after the optimization.

### One discovery per gather

Build a fixture containing:

- at least two native Codex sessions;
- one tmux Codex session needing boundary reconciliation;
- repeated facts for a native row (last message, status/outcome, identity/client, eligibility, binding prompt when applicable);
- optionally one Claude row as a negative control.

Assert:

- the entire `GatherAgents()` call performs one Codex candidate discovery;
- the Claude row still calls its existing resolver and does not enter the Codex index;
- `CodexBindingForSession` and `CompleteCodexBinding` both use the scoped resolver and do not increment discovery count beyond the gather's single enumeration;
- a second `GatherAgents()` call creates a new scope and increments the discovery count once more.

### Concurrency

- Run two resolver scopes concurrently against the same fixture and compare complete results.
- Exercise repeated concurrent calls on a single scope if the scoped reader is shareable within one gather; otherwise assert/document serialized ownership and keep all internal callers serial.
- If practical with existing radar test seams, run concurrent gathers using isolated fixture state; otherwise keep concurrency at transcript scope and rely on full `-race` coverage for radar.
- There must be no global current-resolver variable or mutable shared map.

## Static review checks

- `git diff -- internal/transcript/claude.go` is empty.
- No public JSON struct tags, CLI flags, HTTP schemas, refresh intervals, or native retention constants changed.
- Search changed code for cached values: the scoped index may retain only candidate names and resolved path slices; metadata is read to validate a match but is not retained as session state. No timestamps, boundary kinds, error strings, parsed turns, or status values may be cached.
- Verify every Codex transcript call reachable from `gatherAgents()` either uses the scope or is documented as intentionally outside path discovery.

## Commands

Run in order:

```sh
go test -race ./internal/transcript
go test -race ./internal/radar ./internal/resume ./internal/sessionpolicy
go vet ./...
make lint
go test -race ./...
```

Check formatting with the repository gate or:

```sh
gofmt -l <changed-go-files>
```

Any output from `gofmt -l` is a failure.

## Local runtime verification

Use the built optimization CLI against the same Codex home used for the baseline.

1. Before editing, capture at least five baseline `agents --json` samples; repeat after the implementation with the same `CODEX_HOME` and workload.
2. Start an independent monitor sampling all live processes whose executable or command identifies this gtmux checkout or app bundle at 250 ms or faster.
3. Stop launching additional validation work when the count reaches 9, leaving one-process headroom; terminate the workload if the count reaches 10.
4. Record active Codex session count, per invocation elapsed time and CPU time, exit code, and maximum process count; summarize median and p95 before/after.
5. Confirm JSON is valid and representative normalized fields/order match the baseline.
6. Report structural discovery count separately from timing. Timing is supporting evidence for whether one per-gather enumeration meets the observed workload.

Pass conditions:

- Maximum gtmux process count is `<= 10`; the guard stops new work at 9 and terminates at 10.
- No overlapping/process growth regression appears during the monitored run.
- Candidate discovery is once per Codex-using gather.
- Targeted and full race tests pass.
- User-visible output and Codex state transitions match the baseline.
