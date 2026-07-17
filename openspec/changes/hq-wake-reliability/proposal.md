# Proposal: hq-wake-reliability

## Why

hq-perception-v2 defined the right wake protocol — decision-dense events knock with
one signal line, everything else stays pull-side. But the DELIVERY of that knock is
best-effort in ways that lose whole events, and its dedup/suppression rules were
written for the happy path. A 2026-07-17 read of the wake path found eight defects,
five of which lose an event OUTRIGHT (HQ never learns, and nothing reports the loss).
This change makes the channel reliable: every knock either lands, is retried, or is
surfaced as a degradation — never silently dropped.

### Losses (the wake never reaches HQ)

1. **A failed send DELETES the nudge.** `drainInto` claims each queue file by renaming
   it to `.sending`, sends with `tmux.SendText(pane, text, true)` — and then
   `os.Remove`s the claim **regardless of the send's error**. A tmux hiccup, a pane
   that died between the draft check and the send, `send-keys -l` erroring mid-TUI
   ("not in a mode" — the exact failure `tmux.Paste` was introduced to fix for
   dispatch) — all silently destroy the batch. There is no ack: nothing ever confirms
   the line reached HQ's box, and nothing retries.
2. **A crashed drain strands the batch forever.** The `.sending` name is the claim, and
   the ONLY code that ever touches it again is the `os.Remove` at the end of the same
   function. If the hook process is killed between the rename and the remove (a hook is
   a short-lived process; tmux kills its pane, the machine sleeps), the file keeps the
   `.sending` suffix — `hasPending()` and `drainInto` both only scan `.txt`, so those
   nudges are invisible and undeliverable for the rest of the state dir's life.
3. **goal-changed dedups FOREVER on the prompt head.** `nudgeGoalChanged` skips when the
   marker equals the new head, and the marker is never expired. Typing `继续` into an
   agent window a second time — an hour or a day later — produces the same head, so HQ
   is never told. The same marker doubles as the pane's goal for the `done` wake, so
   the two concerns cannot be tuned apart: making dedup expire would also churn the goal.
4. **Every non-prose submission is dropped silently.** `eventSummary` returns `""` when
   `CleanUserPrompt` rejects the payload, and `nudgeGoalChanged("")` returns early. A
   slash command (`/compact`, `/model`, a custom `/deploy`) IS a user act that changes
   what a session is doing, but it reaches HQ as silence. `isClaudeMetaPrompt` matches
   the loose prefix `<command-` and `stripInjected` does not know the current `» gtmux·`
   wake format (only the retired `[gtmux]` one).
5. **HQ detection is one exact string compare.** `findSupervisorPane` / `findHQPane`
   accept a pane only when `#{pane_current_path} == state.HQHome()`. tmux reports the
   PHYSICAL path; `HQHome()` is built from `$HOME` — so any symlink on the way
   (`~/.config` symlinked into a dotfiles repo is a common setup; `/tmp` → `/private/tmp`
   on macOS) makes every wake resolve "no HQ" and return silently. A `cd` inside the HQ
   pane does the same. And when detection fails, the event is DISCARDED rather than
   queued: nothing is left to deliver when HQ is found again.

### Delay and flooding

6. **The draft-guard backstop is 20s.** A nudge that arrives while the user is typing in
   HQ queues to disk, and its unconditional flush is the serve slow-tick (20s) — the
   ticker that also runs `df`/`ps`/`memory_pressure`. Interactive knocks wait on a
   sampling cadence they have nothing to do with.
7. **The queue has no priority and no cap.** `drainInto` joins EVERY due entry with
   `" · "` into one line. A backlog (HQ away, a drafted box) coalesces unboundedly: a
   `goal-changed` waits behind twenty `resource·warn`s, and a hundred-entry join becomes
   a single multi-kilobyte paste — which is exactly the payload shape that fails to land.

### Flapping (the commander's lived pain)

8. **Bare thresholds re-alert on dither.** `resourceTierKey` dedups by tier, which stops
   40→39→38 GB (all amber) — but not 15.1→14.9→15.1 GB, which crosses the red line each
   way and re-nudges every crossing. `loadTier` is worse: `LoadAmber = 1.0` sits exactly
   where a busy machine's load ratio oscillates. There is no hysteresis (one threshold
   per tier, used for both entry and exit), no confirmation window (a single sample
   commits a tier change), and no minimum restate interval (amber→normal→amber inside a
   minute nudges twice).

## What Changes

Four phases, ordered by "stops losing events" first.

### Phase A — stop the silent drops (P0)

- **`.sending` orphan reclaim.** A drain begins by renaming any `.sending` file older
  than 60s back to `.txt`. A crashed drainer's batch rejoins the queue instead of
  disappearing (defect 2).
- **goal-changed → fingerprint + TTL, goal decoupled.** The dedup marker becomes
  `{hash: sha256(clean prompt), ts}` and suppresses only an identical prompt within
  5 minutes; the pane's goal moves to its own marker that the `done` wake reads (defect 3).
- **Non-prose submissions still wake.** A rejected prompt is CLASSIFIED rather than
  dropped: a slash-command wrapper wakes as `goal:"(slash-command) /compact"`; genuine
  non-user content (harness blocks, gtmux's own echoed wake lines) stays silent.
  `isClaudeMetaPrompt` narrows to the exact wrapper tags, and `stripInjected` learns the
  current `» gtmux·` line format (defect 4).
- **HQ pane resolution hardened + queue-on-miss.** Resolution moves into one shared
  `internal/hqpane` used by both call sites, with three criteria in order: the
  `@gtmux_hq_home` pane option (`gtmux hq` stamps it on spawn — exact and cwd-proof),
  symlink-normalized `pane_current_path`, symlink-normalized `pane_start_path`. A
  successful resolve stamps "HQ seen"; when resolution fails but an HQ was seen within
  2h, the wake is ENQUEUED (delivered by the next drain that finds HQ) instead of
  dropped (defect 5).

### Phase B — ack + retry (P0)

- **Split paste from submit.** `hqnudge` delivers with `tmux.Paste` + `tmux.SendKey(Enter)`
  (the dispatch delivery path) instead of `SendText(…, enter: true)`.
- **A failed send requeues.** Any error from paste/Enter renames the `.sending` claims
  back to `.txt`. Nothing is removed until delivery is CONFIRMED.
- **Ack by screen read.** After Enter, one frame read confirms the batch landed, reusing
  dispatch's normalized head matcher (`dispatch.ContainsHead`, exported for this).
  Unconfirmed → requeue.
- **Batch id for re-send idempotence.** Each coalesced line ends with ` · #<id>` (6 hex
  of the batch's entry identities + payload). A retry of the same batch carries the SAME
  id; the ack looks for exactly that id, and the playbook teaches HQ that a repeated id
  is a re-send to ignore.
- **The retry is generous with errors, bounded with missed acks.** A paste/Enter error
  landed nothing, so it retries without limit. A missed ack may have landed, so the
  entry is re-sent at most twice more and then dropped — an unbounded re-paste loop
  against a TUI we can never read would be worse than the silent drop we're fixing.
- **Consecutive failures escalate.** 3 unconfirmed deliveries in a row raise a CRITICAL
  `wake-degraded` — a control record (important severity), a best-effort HQ line, and a
  desktop notification, since the broken thing IS the channel that would otherwise carry
  the alarm.

### Phase C — flapping (the commander's痛点)

- **Hysteresis.** A tier is ENTERED at its threshold and LEFT only past an exit margin
  (`resource.diskHysteresisGB`, default 2 → red at <15 GB, clear at ≥17 GB;
  `resource.loadHysteresis`, default 0.15 → amber at ≥1.0, clear below 0.85).
- **Confirmation window.** A tier change commits only after `resource.confirmSamples`
  (default 3) consecutive samples agree.
- **Minimum restate interval.** The same tier does not re-nudge within
  `resource.minRestateMinutes` (default 30) — bypassed by an ESCALATION (a strictly more
  severe tier always knocks).

### Phase D — latency and flood (P1)

- **A 3s drain ticker.** A new fast tick in serve's single hub goroutine drains the HQ
  queue, gated on the existing cheap `Pending()` dir scan. The draft-guard backstop drops
  20s → 3s; a quiet queue costs one readdir.
- **Priority + caps.** Each queue entry carries its class priority (0 decision-dense:
  `waiting`/`asks`/`goal-changed`/`crash`/`feed-degraded`/`wake-degraded`; 1 outcome:
  `done`/`resolved`/`new-session`/`reap-suggest`/`tick`; 2 standing:
  `resource·warn`/`limits·warn`). A drain emits at most 8 lines per batch, highest
  priority first, oldest first within a priority, and appends `+N more queued` when it
  holds some back. A queue over 200 entries evicts its lowest-priority oldest entry.

### Recorded, NOT fixed here: the pull-side severity hole

`events.Severity` maps `UserPromptSubmit` to `routine`, yet the playbook
(`hq.go`, `gtmux events --severity important` = "the attention stream") and CLAUDE.md
both tell HQ that the important stream is what to pull. A user's direct instruction to
an agent is thus INVISIBLE to an HQ that follows its own playbook — the wake line is the
only trace, so a missed wake (exactly what this change fixes) means the instruction is
unrecoverable by pull. Two candidate fixes, deliberately deferred to a follow-up so this
change stays about delivery:

- **(a) Raise the severity** of a user-direct `UserPromptSubmit` in a non-HQ pane to
  `notable` (or `important`) — the record is already distinguishable by pane. Risk: the
  attention stream widens for every consumer of the tier.
- **(b) Fix the playbook wording** — `--severity important` is the ESCALATION stream, not
  the whole attention stream; the reconcile pull is `--since-seq <n>` with no severity
  filter. Cheaper and arguably more correct: severity ranks urgency, not relevance.

Recommendation: (b) plus a `notable` bump for user-direct submits — but only after this
change lands, since it touches the same playbook that Phase B is already re-versioning.

## Capabilities

### Modified Capabilities

- `hq-wake-protocol`: delivery reliability — acked delivery with retry and a batch id,
  orphan reclaim, no-drop-on-miss HQ resolution, the goal-changed fingerprint+TTL rule,
  slash-command wakes, priority + caps, the 3s drain backstop, and the `wake-degraded`
  escalation.
- `resource-watch`: hysteresis, a confirmation window, and a minimum restate interval on
  the `resource·warn` nudge (escalations exempt).
- `supervisor-agent`: playbook v3 — the `#<id>` re-send convention, the
  `(slash-command)` goal payload, and the `wake-degraded` class.

## Impact

- `internal/hqnudge` (ack/retry/id, orphan reclaim, priority + caps), new
  `internal/hqpane` (shared, hardened HQ resolution), `internal/hqwake` (class priority),
  `internal/hook` (nudge.go goal-changed rule, summary.go prompt classification, call
  sites → hqpane), `internal/transcript` (prompt classification, tighter meta filter,
  `» gtmux·` echo strip), `internal/dispatch` (export `ContainsHead`),
  `internal/resource` (hysteresis + its config), `internal/app` (fast-tick wiring, the
  tier gate, `wake-degraded` escalation, `gtmux hq` stamps `@gtmux_hq_home`, playbook v3),
  `internal/server` (fast ticker + `OnFastTick` dep), `internal/hqfeed`
  (`wake-degraded` control kind).
- Config (additive, all optional): `resource.diskHysteresisGB`, `resource.loadHysteresis`,
  `resource.confirmSamples`, `resource.minRestateMinutes`.
- Docs: `docs/cli.md` (the `resource` config object), CLAUDE.md (the wake reliability
  semantics).
- No HTTP surface change (`api/contract.md` untouched); no new CLI command; the
  `agents --json` / `digest --json` contracts are unchanged.
- `hqPlaybookVersion` 2 → 3, so the managed AGENTS.md upgrade reaches existing homes on
  the next `gtmux hq` after `gtmux update`.
