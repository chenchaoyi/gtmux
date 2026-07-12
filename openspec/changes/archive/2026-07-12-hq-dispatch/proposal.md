# hq-dispatch — verified programmatic dispatch (`gtmux spawn`) + HQ role constraints in built-in logic

## Why

HQ (中控) can already sense and drive the fleet, but *dispatching new work* is
hand-rolled tmux choreography, and every manual step is a place for a silent
failure. A string of incidents (2026-07-12/13, all now acceptance cases) show the
hole:

1. **Un-proxied launch → 403.** A bare `send-keys "claude"` started an agent
   OUTSIDE gtmux's launch path, so `agentenv`'s auto-proxy never applied and the
   agent 403'd on the model API. Auto-proxy only covers gtmux's OWN launch
   (`hq`/`adopt`/restore) — a hand-typed launch bypasses it. (Rule already noted
   in `hq/knowledge/environment.md`.)
2. **Swallowed Enter, twice.** After pasting a long task, the Enter that submits
   it was eaten by the TUI; the text sat as an unsent draft while HQ believed the
   task was running.
3. **"Empty box + token>0" ≠ working.** HQ read a non-empty token counter and an
   empty input box as "已开工", when in fact only a `"cl"` fragment had been
   submitted — the real task text never landed.
4. **Nav keys corrupted a session.** To page a multi-screen form, HQ sent arrow
   keys into an agent's TUI and flipped it into the tmux session-list view,
   derailing the conversation.
5. **Stale chase — no "resolved" signal.** The HQ nudge fires on "started
   waiting" but never on "stopped waiting". The user answered a blocked agent's
   question DIRECTLY in its window; HQ, never told the wait had cleared, kept
   relaying a now-stale to-do and chasing the user about it.
6. **Text-embedded question missed — HQ senses only menu waits.** A release agent
   asked "放行就装?" in its REPLY TEXT and then went idle. That raises no
   permission/plan menu, so no `Waiting` marker, so no nudge — HQ was blind to a
   turn that clearly needed a decision, and the user sat waiting. HQ must sense
   EVERY turn-end response, not just menu/permission waits.
7. **No lifecycle closure — dispatches pile up.** `spawn` creates sessions and git
   worktrees; nothing reclaims them when the work is done. Finished branches,
   merged worktrees, and idle sessions accumulate until the machine is cluttered.
   Dispatch needs a safe, approval-gated way to reap what it created.
8. **Re-nagged about a kept dispatch.** Once reclaim suggestions exist (⑦), a
   dispatch the user has deliberately DECIDED to keep would be re-suggested on every
   tick — HQ nagging about a decision already made. A declined suggestion must be
   remembered (snoozed) for a bounded window, not re-litigated.
9. **Duplicate `/compact` — no re-send interlock.** HQ, unsure the first `/compact`
   had taken, sent it AGAIN, double-compacting a session. Delivery has no memory of
   what it just sent, so a nervous retry re-fires an identical, side-effecting
   payload. A recent-payload interlock must refuse an identical re-send within a
   window unless explicitly forced.
10. **Single-frame screen misread.** Reading ONE `capture-pane` frame to judge
    landing gave false reads twice — a transient "ctx 319%" and an in-progress
    compact progress bar were misjudged. A one-shot screen read is not evidence:
    verify must prefer deterministic hook events and, where it must read the screen,
    require two consistent frames and locate the input region structurally (not by a
    fixed line offset).

The common root cause: **dispatch is a sequence of un-verified pokes, and HQ is
allowed to do engineering-flavored things (type launches, drive TUIs) it should
never do.** Internalize the operational lessons as built-in machinery so the
hallucination space disappears.

Two blocks (user-decided 2026-07-12):

- **A. Programmatic, self-verifying dispatch** — a new `gtmux spawn` that does the
  whole launch→deliver→**verify** atomically, and the same deliver-verify hardened
  into `gtmux send`. The ONLY success criterion is that the task text landed in
  the conversation record and the agent began responding; anything less reports
  `delivered:false` with on-screen evidence — never a silent success.
- **B. HQ role constraints in built-in logic** — the seeded HQ playbook +
  knowledge encode: HQ never does engineering work itself (no code, no builds, no
  repo edits); it only senses (read-only), decides, dispatches (`spawn`/`send`),
  supervises, reports; it never sends navigation keys into an agent TUI; and every
  dispatched task is tracked in a ledger that nudges HQ on done/stuck.

## What Changes

### A — `gtmux spawn` (new) + `gtmux send` verify hardening

- **New capability `agent-dispatch`.** `gtmux spawn [flags] <goal…>` atomically:
  - **Target a pane** — create a fresh detached tmux session (default; opens a
    terminal tab, unfocused, so it never steals the user's focus) OR reuse
    `--pane <id>`.
  - **`--worktree <branch>`** — `git worktree add` for that branch and run the
    agent there (isolated dispatch), auto-removed only by the user.
  - **Launch through `agentenv`** — the launch command is `agentenv.Wrap(…)`, so
    the network proxy is applied by construction (fixes incident ①). `--model`
    passes a model to the agent (e.g. `--model opus`); `--agent` picks which agent.
  - **Wait for ready** — poll the pane until the agent process is actually live at
    its prompt (`classifyAgent` reports a real agent, not a bare shell), bounded by
    a timeout.
  - **Deliver via paste, then verify (LAYERED — deterministic first, screen-read
    only as a fallback).** Put the task text on a tmux paste buffer (`load-buffer` +
    `paste-buffer`), NOT `send-keys -l` (which, mid-TUI, errored `not in a mode`);
    confirm the FULL text is in the input draft (fixes the `"cl"` fragment, incident
    ③); submit Enter; then **verify** the delivery landed. Verification is layered to
    shrink the misjudgment space (incidents ⑨/⑩):
    - **Primary — deterministic hook evidence.** For a hook-equipped agent (Claude
      Code and any agent whose hook carries the prompt), the #388 session-events
      stream is authoritative: a `UserPromptSubmit` event on this pane whose content
      matches what we delivered = landed; `Stop` = the response completed;
      `PreCompact` = a compaction actually began (so a `/compact` is confirmed, not
      re-sent). No screen-scraping needed.
    - **Fallback — hardened screen-read** (hook-less / foreign agents only):
      full-screen capture WITH scrollback margin (never a tail sample); locate the
      **input region by its separator/box line** so "❯ text" is unambiguously draft
      vs submitted; find evidence by **pattern search, not a fixed line offset**; and
      require **two consecutive consistent frames** before declaring "not landed" (a
      single frame misread a transient ctx% and a compact progress bar — incident ⑩).
    - **`queued` is a distinct outcome.** If the agent shows "Press up to edit queued
      messages", the text was accepted but QUEUED behind the current turn — reported
      as `queued`, neither a plain success nor a failure.
    If the draft still holds the text (no submit event, text still in the box), the
    Enter was swallowed → **re-send Enter with backoff** (fixes incident ②). On
    timeout: `delivered:false` + a capture of the current screen as evidence — never
    silent success.
  - **Re-send interlock (fixes incident ⑨).** Before delivering, `spawn`/`send`
    record a hash of the payload per pane; an IDENTICAL payload re-sent within
    `resendWindow` is REFUSED unless `--force`. A nervous duplicate `/compact` (or any
    identical retry) is sealed off by construction, while `--force` keeps a
    deliberate repeat available.
  - **Pre-flight checks** — proxy reachability (which proxy `agentenv` will apply,
    and a warning when a direct launch would 403), resource watermark
    (`preflightResource`, reusing resource-watch), and subscription remaining
    (`gtmux limits`) → an advisory model suggestion when `--model` is omitted.
  - **`--json`** — `{task_id, pane_id, session, delivered, state, evidence}` where
    `state` ∈ `landed | queued | failed` (the deterministic outcome; `delivered` is
    true for `landed`).
- **Dispatch lifecycle closure — `gtmux reap` (new).** The ledger records what each
  `spawn` CREATED (session/window, worktree path, branch). When a tracked task looks
  done (its pane idle-after-work past a threshold and — if it has a branch — that
  branch merged), HQ receives a `reap-suggest` nudge. `gtmux reap <pane|task_id>`
  reclaims it, SAFELY: it checks the worktree is clean and the branch is merged (or
  explicitly abandoned via `--abandon`); on pass it kills the session, `git worktree
  remove`s the worktree, and optionally deletes the merged branch; on FAIL it reports
  what's blocking and touches nothing. `--json` for the outcome. Never auto-runs —
  reclaim is always suggest → user approves → execute. A `reap-suggest` the user
  DECLINES can be **snoozed** (`gtmux reap --snooze <pane|task_id>`): the ledger
  marks the candidate silenced for a configurable TTL (`reapSnoozeTTL`), so HQ stops
  re-suggesting it until the window lapses — no repeated nagging about a dispatch the
  user has chosen to keep.
- **`gtmux send` gains default verify.** For a text send (not `--key`), CLI
  `gtmux send` runs the same layered deliver-verify (returns as soon as it confirms,
  so a healthy send stays fast; `--no-verify` opts out) and honors the re-send
  interlock (`--force` to repeat an identical payload). **`POST /api/send` is
  unchanged** (stays fast for the mobile reply path — user-decided).

### B — HQ role constraints + task ledger (built-in)

- **Modified `supervisor-agent`.** The generated HQ playbook (`hqInstructions`) and
  knowledge seeds encode a hard role boundary:
  - HQ **never performs engineering work** — it does not write code, run builds,
    or change repositories. Its verbs are: **sense** (digest / `capture-pane`,
    read-only) · **decide** · **dispatch** (`gtmux spawn` / `gtmux send`) ·
    **supervise** · **report**.
  - HQ **never sends navigation keys** (arrows / Tab / Page / mode keys) into an
    agent TUI (fixes incident ④). A form it cannot read is `gtmux focus`-ed for the
    user to handle — HQ does not blind-drive it.
  - Dispatch goes through `gtmux spawn` (verified), never hand-rolled `send-keys`
    launches (fixes incident ①). `environment.md` is strengthened: auto-proxy only
    covers gtmux's launch path.
  - **Reclaim is suggest → approve → execute.** HQ proposes reaping a finished
    dispatch (naming the session/worktree/branch and the exact `gtmux reap` command);
    it NEVER auto-deletes. Destructive closure waits for the user's go-ahead (fixes
    incident ⑦). When the user DECLINES a reap suggestion, HQ snoozes the candidate
    (`gtmux reap --snooze`) and does NOT re-suggest it until the snooze lapses —
    "user said keep it" is a decision HQ must remember, not re-litigate every tick.
- **Task ledger, merged into digest + a needs-you ledger (user-decided: the fuller
  version).** `gtmux spawn` records each dispatch (task→pane→goal→model→status).
  The ledger's live status is derived from the pane's radar state
  (delivered → working → waiting → done). It surfaces in the digest (a dispatched
  pane's row carries its task goal + lifecycle status) and as a unified **needs-you
  ledger** — the things awaiting the user/HQ: waiting agents PLUS dispatched tasks
  that finished or stalled — via `gtmux tasks [--json]`. When a tracked task's pane
  transitions to waiting (needs input) or idle-after-work (done), the existing HQ
  nudge channel informs HQ.
- **Waiting-RESOLVED nudge + auto-clear (fixes incident ⑤).** The HQ nudge channel,
  today one-sided (fires on entering `waiting`), gains the other edge: when a pane
  leaves `waiting` for `working`/`idle`, a `resolved` nudge is typed to HQ — with
  the pane and a short summary of the ORIGINAL ask — deduped the same way (one
  resolve per wait, never re-fired). The dispatch ledger entry is **auto-cleared**
  (销账) on that transition. The HQ playbook gains a matching rule: on `resolved`,
  RETRACT any pending relay/chase about that pane — the user or the agent already
  handled it (e.g. the user answered directly in the pane's own window).
- **Turn-end response awareness + triage (fixes incident ⑥).** Building on the
  session-events stream (#388), EVERY turn-end (`Stop`) SHALL emit an event carrying
  a **reply summary** (the tail of the assistant's last reply) and a deterministic
  **class**: `asking` (the reply text ends on a question to the user — the "放行就装?"
  case that raises no menu) vs `report` (a plain turn-end). HQ subscribes to the
  stream (`gtmux events --follow`); the nudge PUSH is reserved for attention-worthy
  turns — an `asking` turn-end (no menu, but a decision is needed) and a tracked
  task's completion — so HQ is not flooded by ordinary progress turns (those live in
  the stream, pull-only). The HQ playbook gains a triage rule: every response event
  is triaged — **contains a question → relay to the user, get the decision, backfill
  the answer to the agent**; **reports completion → acceptance-verify + report**;
  **otherwise → record, don't disturb**. (Deterministic plumbing gives HQ the summary
  + the coarse `asking`/`report` flag + the resulting state; the finer 完成-vs-继续
  judgment is HQ's to read from the summary — that is what HQ is for.)

## Capabilities

### New Capabilities
- `agent-dispatch`: verified programmatic dispatch (`gtmux spawn`) — targeted
  launch (new session / reuse pane / `--worktree`), proxy-by-construction, wait-for-ready,
  paste-buffer delivery with land-verify + backoff Enter, pre-flight checks, the
  dispatch ledger (recording what was created), and safe approval-gated reclaim
  (`gtmux reap`). Plus `gtmux send` default verify (CLI) and `gtmux tasks`.

### Modified Capabilities
- `supervisor-agent`: the playbook encodes HQ's role boundary (no engineering
  work; no TUI nav keys; dispatch via spawn/send), the dispatch-ledger nudge on
  task done/stuck, the **waiting-resolved** nudge + retract-stale-chase rule, the
  **turn-end triage** rule (question→relay / completion→accept / else→record), and
  the **reclaim = suggest→approve→execute** rule (never auto-delete);
  `environment.md` strengthened on proxy-launch scope.
- `agent-digest`: rows gain the dispatched task's goal + lifecycle status
  (additive), and the needs-you ledger folds in done/stalled dispatched tasks.
- `session-events`: the event `Record` gains additive `summary` + `class` fields
  (every `Stop` emits them, and an `asking` turn-end pushes an HQ nudge), plus a
  `UserPromptSubmit` now records a normalized content head so dispatch verify can
  match a submit deterministically. `PreCompact` is emitted as a lifecycle event so a
  `/compact` can be confirmed from the stream.

## Impact

- **New:** `internal/dispatch` (deliver-verify + ledger, injectable I/O so it's
  unit-testable and cgo-free), `internal/app/spawn.go` (`cmdSpawn`),
  `internal/app/taskscmd.go` (`cmdTasks`), `internal/app/reap.go` (`cmdReap` — safe
  approval-gated reclaim), a `tmux.Paste` primitive, the ledger nudge hook, a
  per-pane recent-send store (re-send interlock). Config: `spawnReadyTimeout` /
  `spawnDeliverTimeout` / `reapIdleThreshold` / `reapSnoozeTTL` / `resendWindow`
  (sane defaults).
- **Touched:** `gtmux send` (default verify), `digest` rows (task fields + ledger),
  the HQ seed (`hqInstructions` + `environment.md`), `app.go` dispatch, help/usage,
  `docs/cli.md` + `CLAUDE.md` command list, `internal/hook/nudge.go` (the resolved
  edge + the `asking` turn-end nudge + ledger auto-clear), `internal/events`
  (`Record.summary`/`class` on Stop, a content head on UserPromptSubmit, a
  `PreCompact` event), and `internal/hook/classify.go` (route PreCompact).
- **Unchanged (deliberately):** `POST /api/send` stays fast; the Swift menu-bar app
  is not touched (its `gtmux send` calls now verify-fast, backward compatible).
- **Cost:** verify is bounded polling of `capture-pane` (cheap, local); it returns
  on first confirmation, so a healthy dispatch adds sub-second latency. cgo-free —
  `tmux` / `git worktree` are shelled out, no new deps.

## Open design forks (resolved 2026-07-12)

- **Command name** → `spawn` (over `dispatch`/`task`).
- **`send` verify scope** → CLI `gtmux send` defaults verify; `POST /api/send`
  stays fast (no synchronous retry loop) to protect mobile reply latency.
- **Task ledger depth** → the fuller version: merged into digest rows + a
  unified needs-you ledger (not a minimal standalone list).
