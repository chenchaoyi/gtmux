# agent-dispatch Specification

## Purpose
TBD - created by archiving change hq-dispatch. Update Purpose after archive.
## Requirements
### Requirement: Verified programmatic dispatch (`gtmux spawn`)

The system SHALL provide `gtmux spawn [flags] <goal…>`, which atomically launches
a coding agent and delivers a task to it with verification. It SHALL: target a
pane (create a fresh detached tmux session by default, or reuse `--pane <id>`);
optionally create an isolated git worktree with `--worktree <branch>` and run
there; launch the agent through the shared launch path so the CONFIGURED proxy
(when set — the choice is explicit, never probed) is applied by construction;
accept `--model` to
select the agent's model and `--agent` to select the agent; wait until the agent
is actually live at its prompt before delivering; deliver the task text; and
report the outcome. `--json` SHALL emit `{task_id, pane_id, session, delivered,
state, evidence}` where `state ∈ landed | queued | failed | refused-duplicate` and
`delivered` is true only for `landed`. When delivery is not verified, `spawn` SHALL
exit non-zero.

#### Scenario: Launch applies the configured proxy by construction

- **WHEN** `gtmux spawn` launches an agent and a proxy is configured (`on`/`<url>`)
- **THEN** the launch command is wrapped with that proxy env (same rule as
  `gtmux hq`/`adopt`), so a proxy-needing network never 403s from an un-proxied
  launch; when the proxy is `off` the launch is bare

#### Scenario: Reuse an existing pane

- **WHEN** `gtmux spawn --pane <id> <goal>` runs and the pane already hosts a live agent
- **THEN** no new session is created and the task is delivered into that pane

#### Scenario: Isolated worktree dispatch

- **WHEN** `gtmux spawn --worktree <branch> <goal>` runs
- **THEN** a git worktree for that branch is created and the agent runs there

### Requirement: Delivery via paste buffer, not literal send-keys

The system SHALL deliver task text using a tmux paste buffer (`load-buffer` then
`paste-buffer`), NOT `send-keys -l`. Delivery and submission (Enter) SHALL be
separate steps so verification can run between them and re-submit independently.

#### Scenario: Task text is pasted, then submitted separately

- **WHEN** a task is delivered to a pane
- **THEN** the text is loaded into a tmux buffer and pasted into the pane, and
  Enter is sent as a distinct, separately-verifiable step

### Requirement: Layered verification — deterministic evidence before screen-reading

Delivery verification SHALL be layered to minimize misjudgment. For a hook-equipped
agent (one whose session-events stream records prompt submissions — e.g. Claude
Code), the system SHALL treat the stream as authoritative: a `UserPromptSubmit`
event on the pane whose recorded content head matches the delivered text CONFIRMS the
landing, and no screen read is required. Screen-reading SHALL be used only as a
FALLBACK for agents that emit no such event (or when the event does not arrive within
a short grace). The fallback SHALL be hardened: it SHALL capture the full screen with
scrollback margin (never a tail sample), locate the input region STRUCTURALLY (by its
separator/box line, so "❯ text" is unambiguously draft vs submitted), find evidence by
PATTERN SEARCH rather than a fixed line offset, and require TWO consecutive consistent
frames before declaring a delivery not-landed (a single frame has misread a transient
context-usage figure and an in-progress compaction bar).

#### Scenario: A hook-equipped agent confirms from the stream

- **WHEN** a task is delivered to a Claude Code pane and a `UserPromptSubmit` event
  with a matching content head appears on the stream
- **THEN** the delivery is confirmed landed WITHOUT reading the screen

#### Scenario: A single transient frame is not a verdict

- **WHEN** the fallback screen-read sees one frame that would read as "not landed"
  (e.g. a transient context-usage figure or a compaction progress bar)
- **THEN** no failure is declared until a second capture agrees

### Requirement: Landing is the only success; fragments and swallowed Enter are handled

Delivery SHALL be considered successful ONLY when the delivery is confirmed landed
(by the stream, or by the hardened fallback). Before submitting, the system SHALL
confirm the FULL task text (not a prefix) is present in the input draft; a
partial/fragment paste SHALL be retried or reported as failed, never submitted as-is.
A submission whose Enter was swallowed (the text remains in the draft and no submit
event arrived) SHALL be resubmitted with backoff. If verification does not succeed
within the timeout, the system SHALL report `delivered:false` (`state:"failed"`)
together with on-screen evidence (a capture of the pane) and SHALL NOT report success.

#### Scenario: Fragment is not silently accepted

- **WHEN** only a prefix of the task text lands in the input draft (e.g. `"cl"`)
- **THEN** the paste is retried, and if it still cannot place the full text the
  result is `delivered:false` with evidence — never a claimed success

#### Scenario: Swallowed Enter is retried

- **WHEN** the task text is pasted but the submitting Enter is swallowed (the text
  remains in the draft and no submit event appears)
- **THEN** Enter is re-sent with backoff until the delivery is confirmed, or the
  timeout yields `delivered:false` + evidence

#### Scenario: Empty box without a submit is not "working"

- **WHEN** the input box is empty but no submission was confirmed (nothing actually
  entered the conversation)
- **THEN** the result is `delivered:false` — an empty box plus a nonzero token
  counter is NOT accepted as evidence of work

#### Scenario: Timeout never reports success

- **WHEN** verification does not confirm within the deliver timeout
- **THEN** the result is `delivered:false` with a capture of the current screen

### Requirement: A queued submission is reported distinctly

The system SHALL report `state:"queued"` when a submission is accepted but QUEUED
behind the current turn (the agent shows a "queued messages" indicator) — a distinct
outcome that is neither a plain success nor a failure — so a caller can tell "will run
after the current turn" from "landed now" and from "failed".

#### Scenario: Queued is not conflated with landed or failed

- **WHEN** a delivered message is accepted into the agent's queue rather than run
  immediately
- **THEN** the result state is `queued`, distinct from `landed` and `failed`

### Requirement: Re-send interlock refuses an identical duplicate

Before delivering, the system SHALL record a hash of the payload per pane. An
IDENTICAL payload delivered to the same pane within a configurable `resendWindow`
SHALL be REFUSED (delivering nothing, `state:"refused-duplicate"`) unless an explicit
`--force` is given. This seals off a nervous duplicate of a side-effecting command
(e.g. a second `/compact`) while leaving a deliberate repeat available via `--force`.
The interlock SHALL NOT block a different payload, nor a repeat after the window lapses.

#### Scenario: A duplicate within the window is refused

- **WHEN** the same payload is delivered to the same pane twice within `resendWindow`
  and `--force` is not given
- **THEN** the second delivery is refused (`state:"refused-duplicate"`) and nothing is
  sent

#### Scenario: Force overrides the interlock

- **WHEN** the same payload is delivered again with `--force`
- **THEN** the delivery proceeds

#### Scenario: A different payload is unaffected

- **WHEN** a different payload is delivered to the same pane within the window
- **THEN** it is delivered normally

### Requirement: Pre-flight checks before dispatch

`gtmux spawn` SHALL run advisory pre-flight checks that warn but never block:
proxy reachability (which proxy the launch will apply, and a warning when a direct
launch would 403), machine resource watermark (the resource-watch red-line
pre-flight), and subscription-window remaining (`gtmux limits`) yielding a model
suggestion when `--model` is omitted. An explicit `--model` SHALL never be
overridden by the suggestion.

#### Scenario: Red-line resource warns before adding load

- **WHEN** `gtmux spawn` runs while a machine resource is at its red line
- **THEN** it warns (and proceeds) rather than silently piling on load

#### Scenario: Model suggestion is advisory only

- **WHEN** `gtmux spawn` runs without `--model` and subscription room is tight
- **THEN** it prints a model suggestion but launches with the agent's default; a
  provided `--model` is used verbatim

### Requirement: `gtmux send` verifies delivery by default (CLI)

The CLI `gtmux send <pane> <text…>` SHALL verify a text delivery by default using
the same land-verification, returning as soon as delivery is confirmed (a healthy
send stays fast) and reporting failure with evidence otherwise. `--no-verify` SHALL
opt out; `--key` (a single control key) SHALL be unaffected. `POST /api/send` SHALL
remain unchanged (no synchronous verify loop), preserving mobile reply latency.

#### Scenario: A verified text send confirms or reports

- **WHEN** `gtmux send <pane> <text>` runs without `--no-verify`
- **THEN** it confirms the text landed and the agent responded, or reports failure
  with evidence — it does not silently assume success

#### Scenario: The mobile reply path is not slowed

- **WHEN** a reply is sent through `POST /api/send`
- **THEN** it behaves as before (no added synchronous verify/retry loop)

### Requirement: Dispatch ledger and needs-you view

The system SHALL record each `gtmux spawn` dispatch (task id → pane → goal → model
→ status), INCLUDING what the dispatch created (its session/window, worktree path,
and branch, for later reclamation) AND an additive `source` field
(`hq-dispatched` | `user-direct` | `agent-self`), and expose `gtmux tasks [--json]`.
`gtmux spawn` SHALL stamp `source: "hq-dispatched"`; `user-direct`/`agent-self`
entries are ones HQ back-fills from work it sensed (gtmux does not fabricate them).
A ledger entry's lifecycle status (delivered → working → waiting → done) SHALL be
derived from the dispatched pane's live radar state. `gtmux tasks` SHALL lead with
entries needing attention (a tracked pane that is waiting or done-after-work), the
same needs-you-first ordering the digest uses. The `source` field is additive and
optional — an entry without it is treated as `hq-dispatched`.

#### Scenario: A dispatch is tracked

- **WHEN** `gtmux spawn <goal>` succeeds
- **THEN** a ledger entry exists for it with `source: "hq-dispatched"` and
  `gtmux tasks` lists it with its live status

#### Scenario: Needs-you ordering

- **WHEN** `gtmux tasks` runs and a tracked pane is waiting or done-after-work
- **THEN** that entry is listed ahead of still-working ones

#### Scenario: Source round-trips

- **WHEN** a ledger entry is written with a `source` and read back
- **THEN** the same source is returned; a legacy entry without one reads as
  `hq-dispatched`

### Requirement: Safe, approval-gated reclamation (`gtmux reap`)

The system SHALL provide `gtmux reap <pane|task_id>` to reclaim what a dispatch
created. It SHALL run a safety gate FIRST: the worktree must be clean and the branch
merged, unless `--abandon` explicitly overrides. When the gate fails, `reap` SHALL
report exactly what blocks it (uncommitted changes / unmerged commits) and make NO
changes. When the gate passes, it SHALL kill the dispatch's tmux session/window,
`git worktree remove` the worktree, and delete the branch only when merged (or
`--abandon`) and not `--keep-branch`, then clear the ledger entry. `reap` SHALL
never run automatically — only when invoked. `--json` SHALL report the outcome
(`reaped`, plus any `blocked_by`).

#### Scenario: Dirty worktree is report-only

- **WHEN** `gtmux reap` targets a dispatch whose worktree has uncommitted changes
  (and `--abandon` is not given)
- **THEN** it reports the uncommitted changes and reclaims nothing

#### Scenario: Unmerged branch is report-only

- **WHEN** `gtmux reap` targets a dispatch whose branch is not merged (and
  `--abandon` is not given)
- **THEN** it reports the unmerged state and reclaims nothing

#### Scenario: Clean and merged reaps safely

- **WHEN** `gtmux reap` targets a dispatch whose worktree is clean and branch merged
- **THEN** it kills the session, removes the worktree, deletes the merged branch
  (unless `--keep-branch`), and clears the ledger entry

### Requirement: Reclaim suggestion when a dispatch looks done

The system SHALL suggest reclamation to a live HQ when a tracked dispatch's pane is
idle-after-work past a threshold AND its branch is merged (or it has no branch) AND it
is not currently snoozed — a `reap-suggest` nudge naming the session/worktree/branch
and the exact `gtmux reap` command, deduped and gated on a live HQ. The suggestion
SHALL be informational and perform no reclamation itself.

#### Scenario: A finished dispatch is suggested for reclaim

- **WHEN** a tracked dispatch is idle-after-work past the threshold with a merged (or
  absent) branch and an HQ pane is live
- **THEN** HQ receives a `reap-suggest` nudge with the reclaim command, and nothing
  is deleted until the user runs `gtmux reap`

### Requirement: Snooze a declined reap candidate

The system SHALL let the user silence a reap suggestion without reclaiming anything:
`gtmux reap --snooze <pane|task_id> [--for <dur>]` SHALL stamp the ledger entry with a
future `snooze_until` (`now + reapSnoozeTTL` by default; `--for` overrides) and make NO
destructive change. While `snooze_until` is in the future, the reclaim-suggestion check
SHALL skip that entry, so HQ does not re-suggest a dispatch the user chose to keep. The
suggestion SHALL resume once the snooze lapses. The snooze SHALL be cleared when the
entry is reaped.

#### Scenario: Snoozing silences the suggestion, reclaims nothing

- **WHEN** `gtmux reap --snooze <task>` runs
- **THEN** the ledger entry gains a future `snooze_until` and nothing is killed, removed,
  or deleted

#### Scenario: A snoozed candidate is not re-suggested until it lapses

- **WHEN** the reclaim-suggestion check runs for a candidate whose `snooze_until` is in
  the future
- **THEN** no `reap-suggest` nudge fires for it; after `snooze_until` passes, the
  suggestion resumes

### Requirement: Reclaim a bare pane not in the ledger

`gtmux reap` SHALL reclaim a manually-created window that has no ledger entry, given a
bare pane id: it derives the repo context from the pane's cwd (the enclosing git
worktree + its branch) and applies the SAME safety gate as a ledgered reap — the
worktree must be clean and the branch merged unless `--abandon` overrides — then kills
the window, removes the worktree, and deletes the merged branch. When the gate fails it
reports exactly what blocks it and changes nothing. This closes the gap where
`gtmux reap <pane>` reported "no dispatch" for a hand-made window, leaving it
un-reclaimable.

#### Scenario: A manual window is reclaimed under the safety gate

- **WHEN** `gtmux reap <pane_id>` targets a window with no ledger entry whose worktree
  is clean and branch merged
- **THEN** it reclaims the window/worktree/branch, the same as a ledgered dispatch

#### Scenario: A dirty manual window is report-only

- **WHEN** the bare pane's worktree has uncommitted changes (and no `--abandon`)
- **THEN** it reports the changes and reclaims nothing

### Requirement: Dispatched work is self-describing in tmux

`gtmux spawn` SHALL set the created window/pane title to a task slug so a glance at
tmux reads the fleet. The slug SHALL be derived as: an explicit `--title`, else the
worktree/branch slug, else a normalized head of the goal. One feature maps to one
worktree by convention.

#### Scenario: A spawn names its window

- **WHEN** `gtmux spawn <goal>` creates a window
- **THEN** the window/pane title is the task slug (`--title`, else worktree/branch, else
  goal head)

### Requirement: Headless dispatch for background heavy work

`gtmux spawn --headless` SHALL dispatch heavy or batch work (a build, a bulk edit)
WITHOUT popping a terminal tab, and SHALL mark its window as background so a glance at
tmux distinguishes it from windows the user should watch — while keeping the dispatch
fully proxied, land-verified, tracked, and reapable (its pane still exists; "headless"
means no terminal tab and out of the way, not untracked). This lets HQ offload heavy
work without parking its main input loop.

#### Scenario: Heavy work runs without a terminal tab

- **WHEN** `gtmux spawn --headless <goal>` runs
- **THEN** no terminal tab is opened, the window is marked background, and the dispatch
  is proxied, verified, tracked, and reapable like any other

