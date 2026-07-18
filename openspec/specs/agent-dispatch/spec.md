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

- **WHEN** `gtmux spawn` launches an agent and a proxy URL is configured
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
`paste-buffer`), NOT `send-keys -l`. The paste SHALL be BRACKETED (`paste-buffer
-p`), so that an agent TUI receives a multi-line payload as one insertion into its
input box: sent raw, every newline reaches the TUI as a bare Return and submits the
line then and there, splitting one instruction into several messages. Delivery and
submission (Enter) SHALL be separate steps so verification can run between them and
re-submit independently. Because `paste-buffer -p` brackets only when the
application has bracketed-paste mode enabled at that instant, the system SHALL NOT
rely on bracketing alone for atomicity: it SHALL confirm the input draft holds the
delivery before sending Enter (see "Landing is the only success"), so a paste that
streamed raw newlines (submitting a line early) or left an unterminated paste state
(which would make a later Enter insert a newline instead of submitting) is detected
as a draft that does not hold the full text, and is retried or reported — never
submitted as a fragment. This applies to EVERY text-into-a-pane path — the verified
dispatch, `gtmux send` with verification skipped, and `POST /api/send` — which differ
only in whether they confirm the LANDING after submit, not in whether they confirm
the DRAFT before it.

#### Scenario: Task text is pasted, then submitted separately

- **WHEN** a task is delivered to a pane
- **THEN** the text is loaded into a tmux buffer and pasted into the pane, and
  Enter is sent as a distinct, separately-verifiable step

#### Scenario: A multi-line instruction is one message, not one per line

- **WHEN** a delivery whose text contains newlines is pasted into an agent pane
- **THEN** the whole text lands in the input draft as a single unsubmitted block, and
  the separate Enter submits it exactly once

#### Scenario: A paste that did not bracket is not submitted as a fragment

- **WHEN** a multi-line paste arrives while the agent's bracketed-paste mode is off,
  so newlines submit early and the draft no longer holds the full payload
- **THEN** the draft fails the full-content check and Enter is not sent against the
  partial draft — the delivery is retried or reported, never submitted as-is

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
confirm the FULL task text is present in the input draft — both the leading
fingerprint (head) AND the trailing fingerprint (tail) of the payload, or a TUI's
collapsed-paste placeholder that stands in for a folded large paste; a match on the
head alone (a prefix) SHALL NOT authorize submission, because a half-rendered draft
whose tail has not yet arrived would otherwise be submitted truncated and then
misread as landed. A partial/fragment paste SHALL be retried or reported as failed,
never submitted as-is. A submission whose Enter was swallowed (the text remains in
the draft and no submit event arrived) SHALL be resubmitted with backoff, and each
resubmit SHALL re-confirm the draft STILL holds the full text first — the system
SHALL NOT re-send Enter blindly against a draft that is empty (already submitted) or
no longer matches. If verification does not succeed within the timeout, the system
SHALL report `delivered:false` (`state:"failed"`) together with on-screen evidence
(a capture of the pane) and SHALL NOT report success.

#### Scenario: Fragment is not silently accepted

- **WHEN** only a prefix of the task text lands in the input draft (e.g. `"cl"`)
- **THEN** the paste is retried, and if it still cannot place the full text the
  result is `delivered:false` with evidence — never a claimed success

#### Scenario: A head-only draft is not submitted as the whole task

- **WHEN** the draft shows the payload's first lines (the head matches) but the tail
  has not rendered yet
- **THEN** submission waits for the tail within the settle window; a draft holding
  only the head is treated as a fragment, not submitted as the complete task

#### Scenario: Swallowed Enter is retried, but only against a matching draft

- **WHEN** the task text is pasted but the submitting Enter is swallowed (the full
  text remains in the draft and no submit event appears)
- **THEN** Enter is re-sent with backoff after re-confirming the draft still holds
  the full text; once the draft is empty or no longer matches, no further Enter is
  sent, and the timeout yields `delivered:false` + evidence if never confirmed

#### Scenario: Empty box without a submit is not "working"

- **WHEN** the input box is empty but no submission was confirmed (nothing actually
  entered the conversation)
- **THEN** the result is `delivered:false` — an empty box plus a nonzero token
  counter is NOT accepted as evidence of work

#### Scenario: Timeout never reports success

- **WHEN** verification does not confirm within the deliver timeout
- **THEN** the result is `delivered:false` with a capture of the current screen

### Requirement: A retry never duplicates a delivery

A delivery SHALL place its text in a pane's input draft AT MOST ONCE. Because a paste
appends to whatever the box already holds, the system SHALL re-paste only after the
draft is confirmed EMPTY, and clearing the draft SHALL NOT be assumed to work: the
clear key empties a single line, so a multi-line draft survives it. When the draft
cannot be confirmed empty, the system SHALL report `delivered:false` with evidence
rather than paste again. A paste SHALL be given a settle window to render before the
draft is judged a fragment (the frame immediately after a paste can still show the
pre-paste box), and a draft that already holds the delivery SHALL NOT be pasted into
again.

#### Scenario: A late-rendered paste is not pasted again

- **WHEN** the frame captured right after a paste does not yet show the text, and a
  later frame within the settle window shows it in full
- **THEN** the delivery proceeds to submit that one paste — the text is never pasted
  a second time

#### Scenario: An unclearable draft fails instead of duplicating

- **WHEN** a fragment is in the draft and clearing it leaves text in the box
- **THEN** no further paste is attempted and the result is `delivered:false` with the
  box in the evidence

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
created. It SHALL run a safety gate FIRST: the worktree must be clean and, unless
`--keep-branch` is given, the branch merged — unless `--abandon` explicitly overrides.
"Merged" SHALL NOT be limited to the branch tip being a literal git ancestor of the
default branch (true only for a fast-forward/regular merge): a SQUASH merge (e.g.
GitHub's default) rewrites the branch's commits into one new commit on the default
branch, so it SHALL also be recognized as merged, either via a commit on the default
branch with a tree identical to the branch tip's, or via the branch's associated PR
reporting a MERGED state (through `gh`, when available). When `--keep-branch` is
given, the branch is never deleted, so its merge state SHALL NOT gate the reclaim —
only the worktree-clean check still applies. When the gate fails, `reap` SHALL report
exactly what blocks it (uncommitted changes / unmerged commits) and make NO changes.
When the gate passes, it SHALL kill the dispatch's tmux session/window, `git worktree
remove` the worktree, and delete the branch only when merged (or `--abandon`) and not
`--keep-branch`, then clear the ledger entry. `reap` SHALL never run automatically —
only when invoked. `--json` SHALL report the outcome (`reaped`, plus any `blocked_by`).

#### Scenario: Dirty worktree is report-only

- **WHEN** `gtmux reap` targets a dispatch whose worktree has uncommitted changes
  (and `--abandon` is not given)
- **THEN** it reports the uncommitted changes and reclaims nothing

#### Scenario: Unmerged branch is report-only

- **WHEN** `gtmux reap` targets a dispatch whose branch is not merged (and neither
  `--abandon` nor `--keep-branch` is given)
- **THEN** it reports the unmerged state and reclaims nothing

#### Scenario: Clean and merged reaps safely

- **WHEN** `gtmux reap` targets a dispatch whose worktree is clean and branch merged
- **THEN** it kills the session, removes the worktree, deletes the merged branch
  (unless `--keep-branch`), and clears the ledger entry

#### Scenario: A squash-merged branch is recognized as merged

- **WHEN** a dispatch's branch was squash-merged into the default branch (its tip is
  not an ancestor, but the content landed as one new commit — or the branch's PR
  reports MERGED via `gh`)
- **THEN** `gtmux reap` still recognizes the branch as merged and reaps it

#### Scenario: `--keep-branch` is not blocked by an unmerged branch

- **WHEN** `gtmux reap --keep-branch` targets a dispatch whose worktree is clean but
  whose branch is not merged
- **THEN** it removes the worktree and clears the ledger entry, keeping the branch —
  the merge-state gate does not apply since the branch is not being deleted

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

### Requirement: Delivery drops the pane out of copy-mode first

Before delivering task text to a pane, the system SHALL drop the pane out of any tmux
mode (copy-mode / view-mode). While a pane is in a mode, `paste-buffer` and `Enter`
are interpreted as mode-navigation and never reach the program, so an un-cancelled
delivery is silently swallowed (and can be mis-verified as landed). The system SHALL
exit the mode (`send-keys -X cancel`) before pasting, and SHALL treat exiting as a
no-op when the pane is not in a mode. This applies to the verified delivery path
(`gtmux spawn` and `gtmux send`) AND to the plain write paths (`gtmux send`
`--no-verify`/`--no-enter`/`--key` and `POST /api/send`). Land-verification is
otherwise unchanged.

#### Scenario: A scrolled pane still receives the dispatch

- **WHEN** `gtmux send`/`spawn` delivers to a pane that is in copy/view-mode
- **THEN** the pane is dropped out of the mode before the text is pasted
- **AND** the payload lands in the input box and is verified as landed, not swallowed

#### Scenario: A non-scrolled pane is not disturbed

- **WHEN** delivery targets a pane that is NOT in a mode
- **THEN** no `-X cancel` is sent (it would error "not in a mode")
- **AND** delivery proceeds exactly as before

#### Scenario: The API and plain send paths also exit the mode

- **WHEN** `POST /api/send` or `gtmux send --no-verify`/`--key` writes to a pane in
  copy/view-mode
- **THEN** the pane is dropped out of the mode before the key/text is sent, so the
  input reaches the agent

### Requirement: A dispatch blocked at a startup gate or holding an undelivered draft is never done

The system SHALL classify a dispatched worker that is blocked BEFORE running a turn —
sitting at a startup/permission gate, or holding its pasted-but-unsubmitted goal in the
composer — as `waiting` (needs-you) in `gtmux tasks` and the digest, and SHALL NEVER
report it as `done`. `done` SHALL be reserved for a dispatch whose session actually
completed a turn. The undelivered-draft state SHALL be judged from a COLOR-aware capture
that EXCLUDES the agent's suggested-next-command GHOST text — the dim autosuggestion the
agent renders faint (SGR 2), which needs a key to accept and is NOT user input — so a
composer showing only a ghost suggestion is NOT read as an undelivered draft. The system
SHALL also surface WHY via a kind (`startup` / `draft`).

#### Scenario: `gtmux tasks` shows a stuck dispatch as waiting, not done

- **WHEN** a dispatch's pane is at a startup gate or still holds its undelivered draft
- **THEN** `gtmux tasks` and the digest show it `waiting` (needs-you), not `done`, so a
  supervisor is never told a task finished when not one step ran

#### Scenario: A dim suggested-next-command does not block `done`

- **WHEN** a dispatch's pane completed its turn and its composer shows only the agent's
  faint suggested-next-command ghost text (SGR 2), with no real unsubmitted input
- **THEN** the ghost text is not read as an undelivered draft, so the completion is NOT
  suppressed as a stuck `draft`

### Requirement: Unverified send paths confirm the draft before submitting

The system SHALL confirm that the input draft holds the FULL delivered text (head
AND tail, or a collapsed-paste placeholder) before sending Enter on the unverified
text paths — `POST /api/send` (the phone / menu-bar reply) and `gtmux send` with
verification skipped — using the same draft-content check as the verified dispatch. These
paths SHALL still skip the post-submit LANDED verification (to stay within the
phone's latency budget), so they differ from verified dispatch only in whether they
confirm the landing AFTER submit — not in whether they race paste against Enter. The
pre-submit confirmation SHALL be bounded by the same settle window (a healthy paste
confirms within a frame, so the fast path stays fast); if the window elapses without
a full-draft match, the path MAY still send Enter best-effort but SHALL NOT be
required to report success.

#### Scenario: The phone reply does not race paste against Enter

- **WHEN** a multi-line reply is sent via `POST /api/send` with `enter:true`
- **THEN** the text is pasted and the Enter is withheld until the draft is confirmed
  to hold the full text (or the settle window elapses), so the reply submits as one
  whole message rather than a truncated fragment

#### Scenario: A short single-line send stays fast

- **WHEN** a short line is sent via `POST /api/send` and renders in the draft within
  one frame
- **THEN** the confirmation passes immediately and Enter follows without added delay

### Requirement: A dispatch registers its target pane as awaited

The system SHALL register a dispatch's target pane as AWAITED — a durable marker that
HQ is expecting a completion from that pane — on both dispatch paths: `gtmux spawn`
after a delivered dispatch, and `gtmux send` on a confirmed landing (`landed`). The
unverified `POST /api/send` path SHALL NOT register (it is casual input, not an
HQ-awaited dispatch), and a delivery that does not land SHALL NOT register (no phantom
await). The awaited marker SHALL be cleared when the pane's completion wake fires or
when the pane goes away, so it is a one-shot per dispatch and leaves no stale state.

#### Scenario: A landed `gtmux send` marks the pane awaited

- **WHEN** `gtmux send` confirms its delivery landed on a pane
- **THEN** that pane is marked awaited, so its next completion wakes HQ immediately

#### Scenario: A failed delivery does not mark awaited

- **WHEN** a dispatch's delivery is not confirmed landed
- **THEN** the pane is NOT marked awaited — HQ is not left awaiting a phantom completion

#### Scenario: The await clears on completion

- **WHEN** an awaited pane's completion fires its `done` wake
- **THEN** the awaited marker is cleared, so a later unrelated completion does not
  re-fire an awaited wake unless HQ dispatches to the pane again

