# Design — hq-dispatch

## Deliver-and-verify (the heart of it)

The requirement is uncompromising: **the only success is "the delivery is confirmed
landed"; anything less is `delivered:false` with evidence.** The failure modes to
beat are the `"cl"` fragment (③), the swallowed Enter (②), the false-positive read
of "empty box + token>0" as working (③ again), a nervous duplicate `/compact` (⑨),
and a single-frame screen misread (⑩). The through-line of ③/⑨/⑩ is *don't trust
one screen frame* — so verification prefers deterministic hook evidence and treats
the screen as a hardened, two-frame fallback (below).

### Why paste-buffer, not `send-keys -l`

`send-keys -l` typed a long task mid-TUI errored `not in a mode` and, when it did
land, was the vector for both the fragment and the swallowed Enter. So task text
is delivered as a tmux **paste buffer**:

```
tmux load-buffer -b gtmux-dispatch-<n> -    # text on stdin, byte-exact
tmux paste-buffer -d -b gtmux-dispatch-<n> -t <pane>   # -d deletes the buffer after
```

Delivery and submission are **separate steps** (paste does NOT auto-Enter) so
verify can sit between them and re-send Enter independently. `SendText`
(`send-keys -l`) is kept for short control replies; a new `tmux.Paste(pane, text)`
is the multi-line/large-text path.

### Layered verification — deterministic first, screen-read as fallback

The single hardest thing to get right is *knowing the delivery landed*, and the
history of misjudgments (incidents ③/⑨/⑩) all came from trusting the SCREEN. So
verify is **layered**: prefer deterministic hook evidence; fall to a hardened
screen-read only for agents that emit no hooks.

**`internal/dispatch.Deliver` injectable I/O** (so it is fully unit-testable
without tmux — cgo-free, deterministic on fixtures):

```
type IO struct {
    Capture func() string          // capture-pane (full screen + scrollback margin)
    Paste   func(text string) error
    Enter   func() error
    ClearDraft func() error        // C-u / C-a C-k — a fixed, NON-navigation key we own
    Events  func(sinceTs int64) []Evidence  // recent lifecycle events for THIS pane
    Now     func() int64
}
type Evidence struct { Kind string; Head string; Ts int64 } // Kind: submit|stop|compact
```

`Events` reads the #388 `events.jsonl` filtered to this pane and `Ts >= sinceTs`;
`Head` is the normalized content head recorded on `UserPromptSubmit` (below).

**Flow:**

1. **Interlock (incident ⑨)** — `h := hash(pane+text)`. If the recent-send store
   holds `h` for this pane within `resendWindow` and `!Force` → refuse:
   `{Delivered:false, State:"refused-duplicate"}`, deliver NOTHING. Otherwise record
   `{h, Now()}` after a successful paste.
2. **Pre-snapshot** — `start := Now()`; `before := Capture()`.
3. **Paste + fragment guard (③)** — `Paste(text)`; read the draft from `Capture()`,
   locating the **input region structurally** (below the input separator/box line,
   not a fixed offset). Assert the full text's normalized head sits in the draft. A
   large paste a TUI COLLAPSES to a "[Pasted text +N lines]" placeholder counts as
   present (the placeholder is the draft's proof). Only a genuine PREFIX (the `"cl"`
   fragment) fails → `ClearDraft()` + retry up to `pasteRetries`; still partial →
   `{Delivered:false, Evidence}`.
4. **Submit** — `Enter()`.
5. **Verify (poll up to `deliverTimeout`):**
   - **Primary (hook-equipped agent):** `Events(start)` — a `submit` whose `Head`
     matches our text head ⇒ `State:"landed"` (deterministic; no screen needed). A
     `stop` after that ⇒ the response also completed. A `compact` ⇒ a `/compact`
     landed. This path never misreads a progress bar.
   - **`queued` (④):** the screen shows "Press up to edit queued messages" ⇒
     `State:"queued"` — accepted but behind the current turn; a distinct, reported
     outcome, not a failure.
   - **Fallback (hook-less/foreign agent, or the hook stays silent past `hookGrace`):**
     hardened screen-read — require **two consecutive Captures that agree** (defeats
     the single-frame ctx%/compact-bar misread, ⑩); success = the draft no longer
     holds the text (structurally located) AND its head appears in the history region
     (pattern search, not a fixed line).
6. **Swallowed Enter (②)** — if after the first poll window the draft STILL holds the
   full text and no `submit` event arrived, the Enter was eaten: re-`Enter()` with
   exponential backoff up to `enterRetries`, re-checking (5) each time.
7. **Timeout** — `{Delivered:false, State:"failed", Evidence: tail(Capture())}`.
   NEVER report success on a timeout.

`Deliver` returns `{Delivered bool, State string, Evidence string, Attempts int}`
where `State ∈ landed|queued|failed|refused-duplicate`. A healthy hook-equipped
send confirms on the first `Events` poll (~a few hundred ms after the agent
registers the prompt), so default-verify on `gtmux send` is not perceptibly slower.

The normalized head (collapse whitespace, first ~40 runes) is shared by the
`UserPromptSubmit` event `Head`, the draft assert, and the fallback history match,
so the same content survives TUI re-wrapping across all three.

## `gtmux spawn` flow

```
gtmux spawn [--pane <id>] [--worktree <branch>] [--model <m>] [--agent <cmd>]
            [--cwd <dir>] [--no-open] [--timeout <dur>] [--json] <goal…>
```

1. **Pre-flight (advisory, never blocks):**
   - Proxy: resolve what `agentenv` will apply on this network; if `agentProxy` is
     effectively off AND we cannot reach the model host directly, warn that a
     launch would 403 (incident ①). (Cheap: reuse `agentenv`'s port probe; a direct
     reachability check is best-effort.)
   - Resource: `preflightResource()` (resource-watch) — warn at the red line.
   - Limits: `gtmux limits` remaining → if `--model` omitted, print a model
     suggestion (advisory only; never overrides an explicit `--model`).
2. **Target the pane:**
   - `--pane <id>` → reuse (must exist; if it already runs an agent, skip launch).
   - else → `tmux new-session -d` (reuse `newSessionArgs`); `--worktree <branch>` →
     `git worktree add <path> <branch>` first and create the session `-c <path>`
     (worktree root under `$GTMUX_WORKTREE_DIR` or a sibling `<repo>-wt/<branch>`).
   - Open a terminal tab (unfocused) unless `--no-open` — HQ dispatches without
     stealing the user's focus.
3. **Launch:** `cmd := agentenv.Wrap(agentLaunch(agent, model))`; deliver `cmd`
   into the fresh shell (`SendText(pane, cmd, true)`) — proxy applied BY
   CONSTRUCTION (fixes ①). `agentLaunch` maps `--model` to each agent's flag
   (claude: `--model <m>`; others per a small table, default: no model flag).
4. **Wait for ready:** poll `classifyAgent(pane)` until it reports a live agent
   (idle/running/working), not a bare shell, bounded by `readyTimeout`. Times out →
   `delivered:false` + evidence.
5. **Deliver:** `dispatch.Deliver(pane, goal, …)`.
6. **Record:** write the ledger entry (below).
7. **Output:** human summary or `--json {task_id, pane_id, session, delivered, evidence}`.

## Dispatch ledger + needs-you ledger (the fuller version)

- **Store:** `~/.local/share/gtmux/tasks/<task_id>.json`:
  `{id, pane, session, agent, model, cwd, worktree, branch, created_at, delivered,
  status}`. `session`/`worktree`/`branch` record what `spawn` CREATED (for `reap`).
  `status` is DERIVED from the live pane radar state on read
  (`delivered → working → waiting → done`), not written eagerly — the pane state is
  the source of truth, the record just names the dispatch.
- **`gtmux tasks [--json]`** lists tracked dispatches with live status. It is the
  **needs-you ledger**: it leads with tasks that need attention — a tracked pane in
  `waiting` (needs input) or `idle`-after-work (`done`, review me) — the same
  "needs-you first" ordering the digest uses. Waiting agents that were NOT
  dispatched still surface via the digest; the ledger adds the dispatched ones and
  their goals.
- **Digest integration:** a digest row whose pane has a ledger entry carries the
  dispatched `goal` + lifecycle `status` (additive fields, empty otherwise).

## Dispatch lifecycle closure — `gtmux reap` (incident ⑦)

The ledger records what `spawn` CREATED — `session`, `worktree` (path), `branch` —
so reclamation targets exactly what it made, never something the user set up.

- **Reclaim suggestion:** a tracked task is a reap CANDIDATE when its pane is
  idle-after-work past `reapIdleThreshold` AND, if it has a `branch`, that branch is
  merged (`git branch --merged` / an ancestor of the default branch) AND it is NOT
  snoozed (below). On that edge a `reap-suggest` nudge goes to HQ naming the
  session/worktree/branch + the exact `gtmux reap` command. Evaluated where the other
  tick/hook transitions are; deduped.
- **Snooze a declined candidate (incident ⑧):** when the user declines a suggestion,
  `gtmux reap --snooze <pane|task_id> [--for <dur>]` stamps `snooze_until` on the
  ledger entry (`now + reapSnoozeTTL`, default e.g. 24h; `--for` overrides). The
  candidate test skips any entry whose `snooze_until` is in the future, so HQ stops
  re-suggesting a dispatch the user chose to keep; the suggestion resumes only after
  the TTL lapses (a keep is a decision with a shelf-life, not a permanent mute). The
  stamp is cleared when the entry is actually reaped or when the user re-runs
  `--snooze --for 0`. (`now` comes from the caller — the ledger stores the absolute
  `snooze_until`; the tick comparison reads the current time where it already reads
  pane mtimes.)
- **`gtmux reap <pane|task_id> [--abandon] [--keep-branch] [--snooze [--for <dur>]] [--json]`**
  — the SAFE, approval-gated executor (`--snooze` is the non-destructive "keep it,
  stop suggesting" path — it only stamps the ledger, reclaims nothing):
  1. **Safety gate (report-only on fail):** the worktree must be clean
     (`git status --porcelain` empty) and the branch merged — UNLESS `--abandon` is
     given (an explicit "throw it away"). If the gate fails, print exactly what
     blocks (dirty files / unmerged commits) and touch NOTHING.
  2. **Execute on pass:** kill the tmux session/window; `git worktree remove` the
     worktree (`--force` only under `--abandon`); delete the branch only if merged
     (or `--abandon`) and not `--keep-branch`; clear the ledger entry.
  3. `--json` reports `{reaped, session, worktree, branch, blocked_by}`.
- **Never automatic.** `reap` only runs when invoked; HQ SUGGESTS it and waits for
  the user (policy below). The suggestion nudge is informational — no side effects.

## Nudge — both edges + auto-clear (incident ⑤)

Today `nudgeSupervisor` fires only on a fresh `Waiting`. The channel becomes
two-sided, driven from the hook's state transitions (`internal/hook`):

- **Enter waiting** (unchanged): `[gtmux] waiting·<kind> <loc> (<pane>) — <title>`.
- **Resolved** (NEW): when `applyState` is about to `clearWaiting` AND a waiting
  marker actually EXISTED (the transition edge — not a no-op clear), read the
  marker's kind first, then type
  `[gtmux] resolved <loc> (<pane>) — was <kind> "<ask/goal head>"`.
  This fires on the `UserPromptSubmit` / `Resumed` / `Stop` edges — including the
  exact incident ⑤ case: the user answers in the pane's OWN window →
  `UserPromptSubmit` → `clearWaiting` → resolved nudge to HQ.
- **Dedup:** the marker's existence IS the dedup — it is removed on the edge, so a
  second clear is a no-op and no second resolve fires. Same one-shot guarantee the
  waiting side has.
- **Auto-clear (销账):** on the resolved edge, the dispatch/needs-you ledger flag
  for that pane is settled — a `waiting`→`working` task returns to `working`; a
  `→idle` (Stop) task becomes `done`. HQ's pending relay for that pane is now moot.
- **Done nudge:** a tracked task's `→idle`(done) also nudges HQ (`[gtmux] done …`)
  so HQ knows to review/close it. Rides the same Stop edge; deduped by the finished
  marker.
- Everything stays gated on a live HQ pane (no HQ → zero cost) and `hqNudge`.

## Turn-end response awareness + triage (incident ⑥)

The nudge fires on menu waits, but a question in the REPLY TEXT ("放行就装?") that
then goes idle raises no menu → today HQ is blind to it. Fix, built on the
session-events stream (#388):

- **`events.Record` gains additive fields** `summary` (the reply tail, same
  extraction as digest's `last`) and `class`. On every `Stop`, the hook fills them.
  A `UserPromptSubmit` additionally records the prompt's normalized `head` in
  `summary` (Claude's UserPromptSubmit payload carries the `prompt`), which the
  dispatch verify matches deterministically. `PreCompact` is passed through as a
  lifecycle event (state-neutral) so a `/compact` is confirmable from the stream.
- **Deterministic `class`:** `asking` when the reply's last non-empty sentence is a
  question directed at the user (ends with `?`/`？`, or a leading interrogative
  after stripping code/quote lines) — else `report`. This is the ONLY split the
  plumbing asserts; distinguishing "completed the whole task" from "did a chunk" is
  not reliable deterministically, so the finer 完成-vs-继续 read is HQ's job from the
  `summary` (HQ is an LLM — that is exactly its role). Keeping the plumbing
  deterministic preserves the digest layer's zero-LLM-token rule.
- **Push vs pull (don't flood HQ):** HQ SUBSCRIBES to the whole stream via
  `gtmux events --follow` (the #388 pull model) — ordinary `report` turn-ends live
  there and do NOT each fire a nudge. The nudge PUSH is reserved for turns that
  can't wait: an `asking` turn-end (`[gtmux] asks <loc> (<pane>) — "<summary>"`) and
  a tracked task's completion. Deduped by the per-turn finished marker (one nudge
  per turn), gated on a live HQ + `hqNudge`.
- **Complementary to the menu path:** a menu question still goes through `Waiting`;
  `asking` covers the no-menu case. A `Stop` clears waiting, so the two never
  double-fire for the same turn.

## HQ playbook (`hqInstructions`) — role boundary

New policy items, bilingual, in the generated-once seed (user owns edits):

- **HQ never does engineering work.** No writing code, running builds, or changing
  repos. Verbs: sense (digest / `capture-pane`, read-only) · decide · dispatch
  (`gtmux spawn` / `gtmux send`) · supervise · report. Engineering is what the
  agents HQ dispatches are for.
- **Dispatch via `gtmux spawn`** (verified) — never a hand-rolled `send-keys`
  launch (bypasses the proxy → 403, incident ①).
- **Never send navigation keys** (arrows/Tab/Page/mode keys) into an agent TUI
  (incident ④). A form/screen HQ cannot read → `gtmux focus` it and ask the user;
  HQ does not blind-drive it.
- **Track every dispatch** in the ledger (`gtmux tasks`); on `done`/stuck the nudge
  tells you. **On a `resolved` nudge, RETRACT** any pending relay/chase about that
  pane — it was already handled (incident ⑤).
- **Reclaim = suggest → approve → execute.** On a `reap-suggest`, PROPOSE reaping to
  the user (name the session/worktree/branch + the `gtmux reap` command); never
  auto-delete. Run `reap` only after the user approves (incident ⑦). If the user
  DECLINES, `gtmux reap --snooze` the candidate and stop re-suggesting it until the
  snooze lapses — don't re-nag about a dispatch the user chose to keep (incident ⑧).
- **Triage every turn-end response** (from `gtmux events --follow` / the `asks`
  nudge): a reply that asks a question → relay it to the user, get the decision,
  and backfill the answer to the agent; a reply reporting completion →
  acceptance-verify and report; anything else → record, don't disturb the user
  (incident ⑥).

## What is deliberately out of scope

- No change to `POST /api/send` (mobile reply stays fast — user-decided).
- No Swift menu-bar changes (its `gtmux send` calls verify-fast, still compatible).
- No new "HQ auto-dispatches on its own" autonomy — `spawn` is a tool HQ (or a
  human) invokes; the decision to dispatch stays with HQ/the user.
