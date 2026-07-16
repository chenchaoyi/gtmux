# Design: hq-perception-v2

## Context

Today's HQ perception stack: hooks append severity-tagged records to
`events.jsonl`; the `hq-feed` daemon tails them into a silent spool HQ is supposed
to background-tail; a handful of typed nudges (`waiting`, `asks`, `done`-of-tracked-
dispatch) land in HQ's input box via `hqnudge` (draft-guarded, coalesced). Receipt
nudges (`resolved`, `goal-changed`) are suppressed whenever the feed *daemon* is
healthy (`feedSupersedesReceipts`). An LLM agent only acts when text reaches its
input box — so everything not typed is effectively invisible until the next wake.
The commander's live HQ additionally runs a pre-feed legacy playbook (skipped by
the seeder), so the prompt side never learned the feed exists.

Established constraints from dogfooding review (the commander's standard):
low token cost, low screen noise (no per-hook forwarding; no thinking spam),
agent-agnostic HQ, goal-aware perception, done gets an immediate judgment,
signals visually distinct from conversation.

## Goals / Non-Goals

**Goals:**
- One deterministic wake channel; everything else reaches HQ by pull.
- Done of ANY session (tracked or user-direct) wakes HQ immediately with enough
  payload to judge from the line alone in one short turn.
- A summary tick with a zero-token empty gate: silence costs nothing.
- Enrollment: every tmux agent session gets a purposeful dossier at HQ start and
  on appearance.
- Playbook v2 reaches EVERY home, including legacy ones (forced migration).
- All mechanics work for a non-Claude HQ (typed lines + CLI pulls only).
- Signal register visually separated from conversation (CC TUI philosophy).

**Non-Goals (later steps of the overhaul):**
- User-presence signal + HQ send interlock (step 2).
- `completed_unviewed` badges / attach recap / In-Review dispositions (step 3).
- Stable-screen delivery contract, detection manifests, OSC/bell fallback,
  context% health, intent-note self-scheduling (later changes).
- Any HTTP/API surface change.

## Decisions

1. **Wake classes.** Immediate (coalesced): `waiting·<kind>`, `asks`, `done`
   (unattended — see 2), `crash` (StopFailure), `goal-changed` (a user-direct prompt in a non-HQ pane),
   `new-session`, `feed-degraded`. Tick: `tick` (summary due). Everything else
   (prompt submissions, working transitions, resolved-with-no-pending-chase)
   never types into HQ — it lands in events/digest for pull. Rationale: done is
   decision-dense (goal complete? asked something? stalled?) and is the
   scheduling moment for dependent dispatches — batching it recreates the
   original complaint (commander-confirmed 2026-07-16).
2. **Done = unattended completion.** A `Stop` fires at EVERY turn end — an
   interactively-driven session Stops once per reply, and waking HQ per reply
   while the human is chatting recreates both the noise and the collision
   problems. The done wake therefore fires only when the turn-end pane is NOT
   the focused pane of an attached tmux client (`#{pane_active}` + attached
   session — deterministic, no new state): you were looking at it, you saw it
   finish. Attended completions count toward the tick tally instead (they still
   appear in the next brief). Config `hqWake.done = unattended (default) |
   always | tick`.
   **Done wake payload.** `» gtmux·done  %14 gtmux:1.2 │ 3m │ goal:"…" │
   tail:"…"` — loc, duration, goal (digest), reply tail (event summary). Target:
   ≥80% of done wakes judged without a drill. Per-pane merge window (default
   2min, config `hqWake.paneMinGapSec`) folds rapid-fire dones of one pane into
   the newest line.
3. **Tick gate is outcome-counting, not time-only.** The serve slow-tick keeps a
   counter of outcome-level changes (done / new-session / session-gone / stall
   suspicion) since the last delivered tick; at interval N (default 10m, config
   `hqWake.tickMinutes`) it delivers `» gtmux·tick seq a-b │ 3 done · 1 new` only
   when count > 0; count == 0 → nothing (no wake, no tokens). An accumulation
   threshold (default 5, config `hqWake.tickBurst`) fires the tick early.
4. **Kill `feedSupersedesReceipts`; wake-line channel replaces receipts.**
   Suppression keyed on the daemon heartbeat measured the producer. The wake
   controller types the compact line ALWAYS (it is the knock); the feed remains
   pull-side data. Consumer freshness (spool consumed-cursor age) only chooses
   between compact line (fresh consumer) and a fuller fallback line (stale/no
   consumer), never silence.
5. **Enrollment is prompt-driven over existing pulls.** No new enrollment
   machinery in Go beyond a `new-session` wake class (SessionStart/first-sight of
   an agent pane). Playbook v2 directs: on start → `gtmux digest --json`, build
   per-session dossiers (purpose/status/owner) on the board; unclear purpose →
   one-time transcript-head read. Goal-aware sensing lives in the dossier, not in
   code.
6. **Playbook v2 + forced migration.** `hqPlaybookVersion` → 2. `seedHQHome`'s
   legacy branch (CLAUDE.md-only home) now migrates: back up CLAUDE.md →
   `CLAUDE.md.bak-legacy`, write managed AGENTS.md + `@AGENTS.md` pointer +
   seed-once LOCAL.md; a note in the backup header tells the user where their old
   edits went. Rationale: the warn-only path provably left the commander's brain
   stale; personalization belongs in LOCAL.md anyway.
7. **Signal register.** Inbound sigil `»` (U+00BB, Latin-1-safe — survives the
   LANG/UTF-8 mangling class that ate ✳) + `gtmux·<class>` + `│`-separated
   columns. Playbook mandates HQ's wake-turn replies open with `⟣` + one of
   ✅/▪/◈/⚠ and stay one line (+ ≤2 indented detail lines for tick briefs);
   conversational replies to the human carry no sigils. A fixture test pins the
   inbound format; the outbound register is prompt-enforced (reviewed, not
   testable).
8. **`gtmux events --since-seq <n>`** gives the pull-on-wake delta (JSON lines,
   seq-filtered). Replaces the background-tail requirement — works for any agent
   that can run a CLI command.
9. **StopFailure → `crash` event** (severity important): hook wiring + event
   class; wake line carries the error head as DATA.

## Risks / Trade-offs

- **Token cost of per-done wakes.** Accepted deliberately (commander): short
  turns, payload-rich lines, per-pane merge, and the tick absorbing bursts keep
  it bounded; `hqWake.doneImmediate=false` demotes done to tick-batch if a fleet
  proves too chatty.
- **HQ mid-turn queueing.** Typed lines during an HQ turn queue as the next
  message (agent-dependent behavior, but true for CC/Codex TUIs); bursts coalesce
  in the queue dir before the box empties. Latency worst-case = current turn
  length — mitigated by short-turn discipline, not eliminated.
- **Forced migration touches user files.** Mitigated: timestamped backup, never
  deletes, LOCAL.md preserved/created, one-line notice printed by `gtmux hq`.
- **Outbound register is prompt-enforced only** — a sloppy model can drift;
  corrections loop + LOCAL.md remain the recourse.
- **`»` collides with blockquote-ish prose** in rare cases; accepted for
  Latin-1 robustness (strictly better than the current unmarked `[gtmux]`).
- **Focus-based attendance is approximate**: multiple attached clients (SSH +
  local) make "focused" ambiguous — any attached client focusing the pane counts
  as attended; worst case a done is deferred to the next tick, never lost.

## Migration

- On upgrade, first `gtmux hq` migrates legacy homes (backup + regenerate) and
  prints what it did. No action needed for managed homes beyond the v2 regen.
- `feedSupersedesReceipts` removal changes no config surface; `hqNudge:false`
  still disables all injection.
- New config keys under `hqWake` are optional with defaults; absence = defaults.
