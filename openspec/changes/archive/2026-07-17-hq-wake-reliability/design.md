# Design: hq-wake-reliability

## Context

The wake channel is three cooperating pieces:

- `internal/hqwake` — builds the line (`» gtmux·<class> │ …`), owns the tick tally and
  the pull stamp. No tmux, no delivery.
- `internal/hqnudge` — delivers the line into the HQ pane, guarding a half-typed draft.
  Owns the persistent disk queue (a hook is a short-lived process, so the queue cannot
  be in-memory).
- `internal/hook` + `internal/app/slowtick.go` — the call sites that decide WHEN to wake.

Everything below keeps that split. The reliability work lands almost entirely in
`hqnudge` (delivery) and in the two call sites' decision rules; `hqwake` gains only a
pure class→priority table.

## Goals / Non-goals

**Goals.** No wake is lost without a report. A knock behind a draft lands within
seconds, not tens of seconds. A dithering resource cannot re-alert. The existing
IRON RULE survives untouched: **nothing is ever typed or Entered into a non-empty HQ
input box.**

**Non-goals.** Guaranteed exactly-once delivery (a TUI screen is not a transactional
sink — we choose at-least-once with an id for the duplicate). Changing what the wake
CLASSES are (hq-perception-v2 settled that). Fixing the pull-side severity mapping
(recorded in the proposal, deferred).

## Decisions

### 1 · At-least-once with a batch id, not exactly-once

A screen has no ack. Confirmation is a screen read, which can produce a false NEGATIVE
(HQ's reply scrolled the line away within the confirm frame) and, rarely, a false
POSITIVE (an identical earlier line still on screen). So delivery is at-least-once and
the DUPLICATE is made cheap to recognise instead of impossible:

```
» gtmux·done  main:1.0 (%14) │ 3m │ goal:"…" · » gtmux·waiting  … · #a3f1c2
```

`id = sha256(claim names + payload)[:6]`. The claim names are the queue filenames
(fixed-width nanos + pid), so:

- a RETRY of an unconfirmed batch reclaims the same files → same names → **same id**;
- a genuinely new batch with identical text has different nanos → **different id**.

The ack searches the capture for the id (short, unique, and — unlike the line's own
head — not shared with any previous wake). `dispatch.ContainsHead` normalizes whitespace
on the haystack, so a TUI re-wrap between the sigil and the id does not defeat the match.

The playbook (v3) gets one rule: *a wake line whose `#id` you have already acted on is a
re-send — ignore it.* That is the whole duplicate story; no state, no dedup engine.

**Why not confirm on the line's head?** Two wakes of the same class about the same pane
have identical heads. The id is the only field guaranteed distinct per batch.

**The two failures are not alike, so the retry policy isn't either.** A paste/Enter
ERROR means nothing reached the pane: retrying is free (no duplicate) and correct, so it
is unbounded — the batch waits for tmux to come back. A missed ACK means the batch may
well have landed: retrying re-pastes a line HQ can already see. If some agent TUI renders
submissions in a way the read can never confirm, an unbounded retry becomes a paste loop
once per drain, forever — strictly worse than the silent drop we started from. So an
unconfirmed entry is re-sent at most twice more (same id) and then dropped, with
`wake-degraded` already raised. A bounded, ANNOUNCED loss is the honest floor.

### 2 · Confirm on the SCROLLBACK capture, not the visible screen

The draft check reads `CapturePane` (visible — the box is at its foot). The ack reads
`CaptureFull` (`-S -200`): HQ starts a turn the moment Enter lands and can push a screen
of tool output within the 300ms confirm gap. 200 lines of margin turns the common
false-negative into a non-event; the id handles what is left.

### 3 · The claim rename is the transaction log

`.txt` → `.sending` is the claim; today the only thing that clears a `.sending` is the
same function that made it. Two changes make the claim recoverable:

- **On failure or non-confirmation:** rename back to `.txt` (the payload is intact on
  disk — nothing was lost, so nothing needs re-deriving).
- **On a crashed drainer:** the next drain reclaims any `.sending` whose mtime is older
  than 60s. 60s is far above a real drain's lifetime (a paste + Enter + one 300ms frame)
  and far below any interval a human notices.

A reclaim can race a live-but-slow drainer: both would deliver the batch. Same id, so
the outcome is the documented duplicate rather than a loss. The old code's failure mode
(silence) was strictly worse.

### 4 · Priority lives in the filename, not a sidecar

The queue is a directory of files claimed by rename; a sidecar index would need its own
crash story. So the name carries everything drain needs:

```
<due-nanos:019d>-<pid>[.p<prio>][.k-<key>][.a<attempt>].txt
```

`dueOf` still parses the prefix before the first `-`, so **entries written by an older
gtmux parse as priority 1 (default) and due-0 (deliver now)** — an in-flight upgrade
drains its backlog rather than stranding it. Order = priority asc, then name asc (time).

Priorities come from `hqwake.PriorityOf(line)`, which parses the class out of the line's
own `» gtmux·<class>` prefix. hqnudge already imports nothing of hqwake's vocabulary;
this is one function and no cycle (`hqwake` imports only `state`).

**Caps.** 8 lines AND ~800 chars per batch (a bounded paste is a paste that lands — and
one an agent TUI won't fold into a `[Pasted text +N lines]` placeholder, which would hide
the id the ack needs; `+N more queued` tells HQ more is coming, and the 3s ticker
delivers it on the next drain). 200 entries per queue, evicting the lowest-priority
oldest — a queue that deep means HQ has been away for hours, and dropping a stale
`resource·warn` beats failing to paste a `goal-changed`.

The attempt counter rides the name too (`.a<n>`), and `batchID` hashes each entry's
identity with that field STRIPPED — otherwise a retry would change the name, change the
id, and defeat the very idempotence the id exists for.

### 5 · HQ resolution: three criteria, one owner, and a queue on miss

Both `hook.findSupervisorPane` and `app.findHQPane` compared `pane_current_path` to
`state.HQHome()` — a physical-vs-logical path compare that a single symlink defeats
silently. New `internal/hqpane` owns the rule for both:

1. the `@gtmux_hq_home` pane option `gtmux hq` stamps at spawn. Exact, survives a
   `cd`, survives every symlink question.
2. `pane_current_path` == home, both sides through `filepath.EvalSymlinks`.
3. `pane_start_path` == home, same normalization — an HQ that `cd`'d away.

(1) only covers panes `gtmux hq` spawned after this ships, which is why (2)/(3) stay.
Normalization is computed once per call and falls back to the raw path when
`EvalSymlinks` fails (a deleted dir), so the rule can only ADD matches, never remove one.

**Queue on miss.** A successful resolve stamps `hqwake/last-seen-hq`. When resolution
fails but that stamp is younger than 2h, the wake is enqueued without delivery: an HQ
that restarts (or a resolution bug we have not found) drains the backlog on its next
empty box. Beyond 2h — no HQ, genuinely — the wake is dropped as before, and the queue
cap bounds the pathological case.

### 6 · goal-changed: fingerprint + TTL, and goal is its own marker

```
goalchanged/<pane>   {"hash":"<sha256 of the clean prompt>","ts":<unix>}   dedup only
goal/<pane>          the clean prompt text (≤400 runes)                    what done reads
```

Dedup fires only on `hash == prior.hash && now-prior.ts < 300`. The hash is over the
FULL clean prompt, not the 40-rune head, so two different instructions sharing a head
are two wakes. A legacy plain-text marker fails the JSON parse → treated as "no prior"
→ one extra wake, once, on upgrade. Acceptable.

Splitting the goal out is what makes the TTL safe: the `done` wake's goal no longer
expires just because the dedup window did.

### 7 · Prompt classification, not a boolean

`transcript.ClassifyUserPrompt(raw) (text, kind)` replaces the `(clean, ok)` boolean at
the decision points:

| kind | payload | wake? |
|---|---|---|
| `user` | the clean prompt | yes — `goal:"…"` |
| `slash` | the command name from `<command-name>` | yes — `goal:"(slash-command) /compact"` |
| `drop` | — | no (harness blocks, gtmux's own echo, the caveat wrapper) |

`CleanUserPrompt` keeps its signature (`kind == user`), so the transcript/chat path is
unchanged — a slash command still does not render as a chat turn. Only the wake decision
uses the finer verdict.

Two tightenings ride along: `isClaudeMetaPrompt` matches the exact wrapper tags
(`<command-name>`, `<command-message>`, `<command-args>`, `<local-command-stdout>`,
`<local-command-stderr>`) instead of the prefix `<command-`; and `stripInjected` drops
`» gtmux·` lines, not just the retired `[gtmux]` format — our own wake line echoed back
must never read as a user goal.

### 8 · Flapping: hysteresis in the sampler, timing in the gate

Two different concerns, deliberately in two places.

**Hysteresis is a property of the thresholds** → `internal/resource`:

```go
func MachineTierSticky(prev Tier, m Machine) Tier {
	if raw := machineTier(m, cfg); raw >= prev { return raw }  // rise: entry thresholds
	if t := machineTier(m, relaxed(cfg)); t < prev { return t } // cleared the exit band
	return prev                                                  // inside the band → hold
}
```

`relaxed(cfg)` moves each threshold AWAY from the alarm (disk +`diskHysteresisGB`, load
−`loadHysteresis`). Rising uses entry thresholds, so **a rise always coincides with a
non-empty raw `Warn` string** — the nudge always has something to say. `Snapshot` stays
pure (the CLI/API/digest keep reporting raw truth; a display that jitters wakes nobody).
Memory has no margin: its tier comes from the kernel's already-discrete pressure level.

**Timing is a property of the alert** → `internal/app/tiergate.go`, a pure state machine
over a persisted `{tier, cand, count, nudged_tier, nudge_at}`:

- an observation equal to the held tier clears any candidate;
- a differing observation must repeat `confirmSamples` (3) times to commit;
- a commit nudges unless `now - nudge_at < minRestateMinutes` (30) — **unless it is an
  escalation** (`rank(new) > rank(nudged)`), which always knocks. A disk that walks
  amber→red must never be silenced by the anti-flap rule.

Only `resource·warn` runs through the gate. `limits·warn` keys on a window identity
(`week (fable)`), where "less severe" has no meaning and a suppressed new-window warning
would be a real loss — it keeps the existing by-tier `markerChanged`.

### 9 · The 3s tick is a new ticker, not a faster slow tick

`slowTickInterval` paces `df`/`ps`/`memory_pressure`/`claude`-spawning limit refreshes.
Nothing about those wants 3s. So serve's hub gets a second ticker (`fastTickInterval`,
3s) and a `Deps.OnFastTick`, wired to a drain that is gated on the existing `Pending()`
readdir. A quiet queue costs one syscall per 3s in a process that already ticks at 2s
for SSE. The slow tick keeps its drain call too — it is the same cheap function, and it
runs once at serve start (before the first fast tick fires).

### 10 · wake-degraded escalates OUT of band

The alarm for "the wake channel is broken" cannot be a wake line. Three carriers, on the
transition into degraded only (`markerChanged`, so recovery does not re-alert):

1. a `gtmux:wake-degraded` control record at `important` severity — the pull side sees it;
2. one best-effort HQ line — costs nothing, and lands whenever only the ACK was flaky;
3. a desktop notification through `internal/notify` — the one carrier that does not
   depend on the broken thing, and the reason CLAUDE.md says a perception outage must
   never stay silent.

## Risks / Trade-offs

- **Every wake now costs a confirm frame** (~300ms + a `capture-pane -S -200`) in the
  hook process. The hook already two-frame-reads for the draft guard; this adds one more
  read on the delivery path only (a queued nudge behind a draft pays nothing).
- **The `#id` suffix is 9 visible characters on every wake line.** It is the price of
  idempotence; the playbook explains it, and it reads as one more `│`-ish field.
- **Duplicates are now possible where none were before** (a false-negative ack, or a
  reclaim racing a slow drainer). Deliberate: the old design's alternative to a
  duplicate was a silent loss.
- **The `@gtmux_hq_home` stamp only helps future spawns.** Existing HQ panes rely on the
  normalized cwd/start-path compare, which is exactly the fix they need.
- **Priority inversion by design:** a `resource·warn` can be evicted at 200 entries.
  Losing a standing warning that will re-fire on the next tick anyway is strictly better
  than losing a `goal-changed` that never re-fires.

## Migration

- Queue files from an older gtmux parse as due-now, priority-1 → drained normally.
- A legacy plain-text `goalchanged/<pane>` marker → one extra `goal-changed` wake, once.
- `gtmux update` + the next `gtmux hq` regenerates AGENTS.md to playbook v3 (backing up
  v2), which is what teaches HQ the `#id` and `(slash-command)` conventions. The code
  behaves correctly against a v2 brain — a duplicate re-send just reads as two lines.
