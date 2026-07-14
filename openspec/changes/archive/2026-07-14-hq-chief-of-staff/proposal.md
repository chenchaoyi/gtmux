# HQ chief-of-staff — persistent posture, decision tiers, graded escalation, learning loop

## Why

The HQ charter (`supervisor-agent` spec) already makes HQ a disciplined **reactive
event forwarder**: it senses nudges, triages turn-ends, dispatches through `gtmux
spawn`, and suggests reclaims. But four gaps keep it from being a real **chief of
staff** (参谋长) with a durable command posture:

1. **No persistent posture.** HQ's operational picture — which ship is on what task,
   under which command mode, its priority/health, what decision is pending, the recent
   lesson — lives only in its context window. A `/compact` or a context reset wipes it.
   HQ re-derives the fleet from scratch every time, and cross-turn continuity is lost.
   Separately, the event *ledger* is complete on disk (`events.jsonl`, append-only,
   rotated) but **undifferentiated**: HQ cannot cheaply ask "what needs attention since
   I last looked" without reading every raw line — which doubles token cost and pushes
   HQ toward reading raw transcripts it was told not to.

2. **No decision-authority model.** The commander (user) interacts through three modes
   — ① dispatch a ship directly, ② adopt HQ's suggestion, ③ discuss, then let HQ decide
   and delegate. But the seed never says *when* HQ may exercise mode ③ autonomy versus
   *must* escalate. So HQ either over-escalates (nagging on reversible trivia) or
   over-reaches (deciding an irreversible/permission/design-fork matter that was the
   commander's to make).

3. **Flat escalation + stale needs-you.** Every attention item reaches HQ the same way,
   so genuinely critical conditions (quota about to run out, a production issue, one
   agent blocking others) get the same volume as routine chatter. And a needs-you HQ
   pushes forward can be **stale** — the pane may have been answered directly in its own
   window between the nudge firing and HQ acting — producing false "needs-you" relays.

4. **Corrections evaporate.** When the commander corrects HQ, or the same footgun is hit
   twice, the lesson is applied once and forgotten. There is no first-class ritual that
   distills a correction/repeat-footgun into a durable charter/KB update — so HQ never
   actually *self-upgrades* from the interaction, which principle H says is its reason
   to exist.

This change closes all four as ONE contract-level upgrade: HQ goes from reactive
forwarder to a chief of staff with **persistent posture, a decision-authority model,
graded + reconciled escalation, and a correction→charter learning loop**.

## What Changes

Four interrelated mechanisms (item 1 is the substrate the rest lean on). Each splits
into a small, deterministic **CODE** core (into gtmux, cgo-free) and **PROMPT/SPEC**
policy (into the single-source agent-neutral seed + specs).

### 1. Persistent situation board + severity-tagged event ledger

- **CODE — event severity (source-stamped).** Add an additive `severity` field to every
  `events.jsonl` record — `routine | notable | important` — computed by a deterministic,
  LLM-free classifier at the **source** (`events.Append`), from fields the record already
  carries (event/state/kind/class). A `Waiting` (needs the user) and an `asking` turn-end
  are `important`; a `report` turn-end / session lifecycle is `notable`; prompt
  submissions and working ticks are `routine`. `gtmux events --severity <level>` filters
  the stream to that level and above, so HQ reads the attention stream — not every raw
  line — and the per-source `summary` (already stamped) means it never reads a raw
  transcript to triage. The write path stays fire-and-forget: a busy or absent HQ never
  blocks the hook.
- **CODE — situation-board scaffold.** `gtmux hq` seeds `notes/board.md` (written only when
  ABSENT, like the knowledge scaffold) — a structured per-ship board (task · command mode /
  source · priority · health · pending decision · recent lesson) HQ maintains and re-reads
  each turn, so its posture survives a context reset.
- **PROMPT.** The seed directs HQ to keep the board current, re-read it after a reset
  before acting, and query `gtmux events --severity important` + `gtmux digest` rather than
  raw transcripts.

### 2. Decision-authority tiers

- **PROMPT/SPEC only.** The seed encodes the three command modes (①direct ②adopt ③discuss→
  HQ-decides) and an explicit autonomy matrix: HQ MAY decide-and-dispatch autonomously
  when the action is **reversible AND low-risk AND within an already-discussed direction**;
  HQ MUST escalate to the commander when the action is **irreversible, touches
  permission/credentials, forks the plan/approach, or falls outside the discussed scope**.
  This makes mode ③ concrete without loosening the existing "never answer a
  permission/plan/design choice" rule.

### 3. Graded escalation + reconcile-before-relay

- **PROMPT/SPEC (leans on §1 severity).** The seed defines graded channels: **routine** →
  situation board only (no interrupt) · **important** → a coalesced summary in the HQ
  session · **critical** → ensure the commander is pushed (the phone). Only genuinely
  critical conditions "ring": quota near-exhaustion (`gtmux limits`/`usage`), a
  production/线上 issue, or one agent blocking others. And a hard **reconcile** step: before
  relaying or escalating any needs-you, HQ re-checks the LIVE `gtmux digest`/`tasks` for that
  pane and DROPS it if the state already moved — killing stale needs-you false positives
  (this complements the existing `resolved` retraction, covering the delayed/queued/
  post-reset case where no `resolved` nudge was observed).

### 4. Correction→charter learning loop

- **CODE.** Seed a `knowledge/corrections.md` topic (and list it in the KB README) — the
  landing place for distilled corrections.
- **PROMPT/SPEC.** The seed makes it a first-class ritual: on a commander correction or a
  repeated footgun, HQ distills the durable lesson — a portable behavior lesson →
  `knowledge/best-practices.md`/`pitfalls.md` (and, when it is charter-level, flag it for a
  seed/spec update); a machine-specific instance → local notes — with the trigger points and
  landing path stated explicitly.

Out of scope (deferred, noted in design): a dedicated HQ-invoked push command (critical→push
rides the EXISTING notification pipeline, which already pushes waiting/attention events to
the phone); a machine-parsed board format (the board is HQ-curated markdown, like the KB);
auto-applied charter edits (a charter-level lesson is *flagged*, the user still lands it).

## Impact

- **Affected specs:** `session-events` (ADDED: severity), `supervisor-agent` (ADDED:
  situation board, decision-authority tiers, graded escalation + reconcile, learning loop;
  MODIFIED: KB scaffold gains `corrections.md`).
- **Affected code:** `internal/events/events.go` (severity field + classifier + stamp),
  `internal/app/eventscmd.go` (`--severity` filter), `internal/app/hq.go` (board scaffold +
  corrections seed + four new playbook sections).
- **Affected docs:** `CLAUDE.md` (the HQ/中控 description).
- **Affected tests:** `internal/events/events_test.go`, `internal/app/eventscmd_test.go`,
  `internal/app/hq_test.go`.
