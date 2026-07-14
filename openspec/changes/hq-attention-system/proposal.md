## Why

Today gtmux types every `[gtmux] …` supervisor nudge as a **visible line into the HQ
pane** — one pipe that both wakes HQ *and* is seen by the user. Low-value chatter
(resolved, send-landed, working ticks) either floods the window or, if HQ stays quiet,
still scrolls past the user. And the feed itself is fragile: if the awareness stream
stalls, nobody notices for a long time — the exact failure the commander called out
("不能挂了半天才知道").

This change splits the two concerns the pipe conflates: **feed HQ everything** (durable,
silent, crash-safe) but **show the user only what HQ judges high-value**. HQ stays
omniscient; the gate sits only on "surface to the user".

## What Changes

- **Split feeding-HQ from showing-user.** The full event stream reaches HQ silently
  (HQ subscribes in the background and reads it); the only user-visible action becomes
  HQ's deliberate *print*. gtmux stops force-typing low-value nudges into the pane.
- **Perception feed with robustness (phase ①, the foundation).** A gtmux-managed,
  no-LLM perception daemon (`gtmux hq-feed`) tails the already-rotated event journal and
  spools it to HQ. It adds what the journal lacks for zero-loss awareness:
  - a **monotonic sequence** on every event (cursor + gap detection),
  - a **consumed cursor** so a crashed/restarted feed resumes with no lost events,
  - a **30 s heartbeat** + a **gtmux-side watchdog** (in the existing `serve` slow-tick,
    no LLM) that mechanically restarts a dead/stale feed and, only after self-heal fails
    twice, escalates,
  - **degradation ⇒ CRITICAL**: any feed downgrade (down / stale / cursor gap) is itself
    surfaced immediately so the user learns of a perception outage in seconds,
  - **startup reconciliation**: on (re)start the feed replays from the cursor and HQ
    pulls one full `digest` snapshot, so a restart never loses state.
- **Severity tiering + attention ledger (phase ②).** Reuse the deterministic event
  `severity` (already stamped `routine|notable|important`) mapped to the design's
  CRITICAL/NORMAL/QUIET surfacing tiers, and grow `gtmux tasks` into a general
  **attention ledger**: per-item tier/priority/surfaced/disposition, **re-orderable**,
  **late-promotion** (a low item promotes when related events accrue), archivable, with
  `--verbose` retro-query.
- **Surfacing config (phase ③).** `gtmux quiet` (a user-tunable quiet mode) + a
  surfacing threshold in `config.json` (e.g. "only CRITICAL", "NORMAL and up").
- **HQ self-check & self-maintenance (phase ④).** gtmux provides deterministic
  self-check TRIGGERS (idle / threshold / daily, sensed no-LLM in the slow-tick); HQ runs
  the actual review-and-cleanup on itself (log bloat, ledger pile-up, memory quality,
  info accumulation) and reports a one-line brief only when it did real work.

## Non-goals

- **Not** a new visible notification channel: CRITICAL still rides the existing
  push/notify pipeline; nothing new is added to the phone contract.
- **Not** an LLM watchdog: all health/timing sensing stays on the gtmux side (cheap,
  deterministic). HQ only *acts* on triggers gtmux raises.
- **Not** changing the event journal's on-disk rotation model (already bounded); only
  additive fields (seq) and a new consumer.
- The persistence-layer *shape* is chosen here (see design.md); the sole hard constraint
  is **rollable** — the journal rotates, the ledger archives; no unbounded single file.

## Capabilities

### New Capabilities
- `hq-attention-system`: the perception feed daemon (monotonic seq, consumed cursor,
  heartbeat, no-LLM watchdog + degradation→CRITICAL, startup reconciliation), the
  attention ledger (tier/priority/surfaced/disposition, re-order + late-promotion +
  archive + `--verbose`), the surfacing policy + `gtmux quiet` config, and the
  self-check TRIGGER machinery (idle/threshold/daily, no-LLM sensing).

### Modified Capabilities
- `session-events`: every event record additionally carries a **monotonic sequence
  number**; the reader/follower supports **resume-from-cursor** so a consumer that
  reconnects replays exactly the events after its last cursor (zero loss, gap-detectable).
- `supervisor-agent`: HQ **subscribes to the silent feed** and gates its OWN output —
  it prints to the user only for CRITICAL/NORMAL, ledger-records QUIET silently; it
  answers confirm-type asks itself within the reversible∧low-risk∧no-fork bound; and it
  runs **self-check/self-maintenance** when gtmux raises a trigger, reporting only on
  real action.

## Impact

- **Code:** `internal/events` (seq, cursor, resume), new `internal/hqfeed` (daemon +
  spool + heartbeat + watchdog logic), `internal/app` (`hq-feed` command, `quiet`
  config, `tasks` ledger extension, slow-tick watchdog + self-check triggers wiring),
  `internal/dispatch` (ledger fields), `internal/hook` (assign seq on append), the HQ
  seed playbook (`internal/app/hq.go` AGENTS.md) for the print-gating + self-check
  behavior.
- **State paths (additive):** `~/.local/share/gtmux/events.seq`, `.../hq-feed/{pid,
  cursor,heartbeat,spool.jsonl}`; `config.json` gains `surfaceTier`/`quiet`.
- **Contracts:** `gtmux events --json` gains an additive `seq` field (non-breaking);
  `gtmux tasks --json` gains additive ledger fields (non-breaking). No changes to the
  radar `agents --json` shape, the notify queue, or the mobile HTTP/SSE contract.
- **Dependency:** the mechanical watchdog runs in `gtmux serve`'s slow-tick; when serve
  is off, HQ's harness (feed-task-exit) + the 5 min polling backstop are the fallback
  (documented in design.md).
