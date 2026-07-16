# Proposal: hq-perception-v2

## Why

Dogfooding surfaced three HQ perception failures with one architectural root: the
hq-attention-system split perception DATA (the silent hq-feed spool) from AROUSAL
(what actually wakes HQ), and arousal became too sparse:

1. **Normal finishes get zero reaction.** `nudgeDone` fires only for tracked
   dispatches (`dispatch.TaskForPane`); a user-direct session that finishes lands as
   a `Stop/notable` record in a spool nothing reads urgently. The user's dominant
   workflow (working directly in agent windows) never surfaces its outcomes.
2. **HQ reacts late, on stale state.** HQ only takes a turn when text lands in its
   input box. When the feed daemon is healthy, `resolved` and `goal-changed`
   receipts are suppressed (`feedSupersedesReceipts`) — which checks the *daemon's
   heartbeat*, not whether HQ actually consumes the spool. Wrong end of the pipe:
   producer-alive ≠ consumer-listening.
3. **HQ collides with the human.** No user-presence signal exists; HQ can `gtmux
   send` into a pane the user is actively driving.
4. **Deployment gap:** `seedHQHome` skips homes with a legacy full CLAUDE.md (no
   AGENTS.md) — the versioned-playbook upgrade never reaches them. The commander's
   live HQ runs a 142-line legacy brain with zero mentions of hq-feed / severity
   tiers / goal-changed / reconcile, while the code side suppresses receipts
   assuming a new brain. Blind on both ends.

The commander's design standard for the fix (2026-07-16): sense every tmux agent
session with an enrollment protocol at HQ start; first-moment awareness at low token
cost; involve the user only in decisions that need them; fast periodic concise
summaries; do NOT forward every hook message to the HQ screen; HQ must work on
non-Claude agents too; perception must be goal-aware, not mechanical task tracking.
Plus (confirmed): done events warrant an immediate wake+judgment each time, and
injected signals must be visually distinct from real HQ conversation (Claude Code
TUI philosophy: one glyph one meaning, stable structure, scannable).

Competitive research (herdr + 18-project landscape survey) confirms: no other tool
has hook-first + screen-fallback layered perception, and 90% of the field hasn't
even noticed the human-collision problem — but also that our done/summary surfacing
lags (Crystal's `completed_unviewed`, official agent-view's attach recap).

## What Changes

Step 1 of the HQ overhaul (further steps — presence/send-interlock, unviewed
badges, stable-screen delivery — are explicitly deferred to later changes):

1. **Wake protocol (code, deterministic).** All typed injections into the HQ pane
   collapse into ONE channel with two classes:
   - **Decision-class, immediate** (coalesced): `waiting·kind`, `asks`, `done`
     (ANY session's idle-after-work, not just tracked dispatches), `crash`
     (StopFailure), `goal-changed` (a user-direct prompt in a non-HQ pane), `new-session` (enrollment),
     `feed-degraded`. A per-pane minimum gap (default 2m) merges pathological
     done-loops.
   - **Tick-class, periodic**: a summary tick fires at most every N minutes
     (default 10) AND only when outcome-level changes accumulated since the last
     tick (done / new session / session gone / stall). Zero changes → no wake, no
     output, no tokens.
   Receipt suppression keyed on daemon heartbeat is REPLACED: the wake line is the
   only knock; the spool/digest stay the pull-side data. Consumer-freshness (HQ's
   consumed cursor) gates any fallback verbosity, not producer heartbeat.
2. **Signal visual language (code + prompt).** Injected lines use a stable sigil +
   columnar format (`» gtmux·done  %14 loc │ 3m │ goal:"…" │ tail:"…"`), and the
   playbook mandates a "signal register" for HQ's wake-turn replies (one line,
   fixed glyph vocabulary ⟣ ✅/▪/◈/⚠, optional indented detail) distinct from its
   conversational prose. Glyph choice is UTF-8-robustness-tested (the ✳/LANG
   mangling class).
3. **Enrollment (建联) protocol (prompt + code support).** On HQ start, the seeded
   playbook directs a full-fleet enrollment: read the digest, build a per-session
   dossier (purpose / status / owner) on the situation board; sessions with unclear
   purpose get a one-time transcript-head drill. Afterwards, a `new-session` wake
   enrolls each newcomer incrementally. Perception becomes goal-aware.
4. **Playbook v2 + forced legacy migration (code).** `seedHQHome` migrates a
   legacy full-CLAUDE.md home (backup → managed AGENTS.md + pointer + LOCAL.md)
   instead of skipping it. Playbook v2 teaches: wake protocol semantics, the
   signal register, graded done responses (silent board note / one-liner /
   escalate), tick briefs (≤6 lines), short-turn discipline, and drops the
   CC-specific "background `hq-feed --tail`" pattern (replaced by pull-on-wake:
   `gtmux events --since-seq <n>` + digest) so any agent can be HQ.
5. **StopFailure sensing (code).** The hook records Claude's `StopFailure` (turn
   died on an API error) as a `crash` event (severity important) so a dead turn is
   never mistaken for a finish.

## Capabilities

### New Capabilities
- `hq-wake-protocol`: the deterministic wake channel into the HQ pane — event
  classes, coalescing, per-pane rate merge, the summary tick with its zero-change
  gate, consumer-freshness gating, and the signal visual language for injected
  lines.

### Modified Capabilities
- `supervisor-agent`: enrollment protocol at HQ start (goal-aware dossiers), the
  signal-register output discipline, graded done judgment, tick briefs, playbook
  v2 + forced legacy-home migration, agent-agnostic operation (no background-tail
  requirement).
- `session-events`: a `crash` event class from StopFailure (important severity);
  `gtmux events --since-seq <n>` delta query used by pull-on-wake.

## Impact

- `internal/hook` (nudge call sites → wake controller; StopFailure handling),
  `internal/hqnudge` (formatting + per-pane merge), new `internal/hqwake` (or
  extension of hqnudge) for tick scheduling in the serve slow-tick,
  `internal/events` (crash class, --since), `internal/app/hq.go` (playbook v2,
  version bump, legacy migration), `internal/hqfeed` (consumer-cursor freshness
  replaces feedSupersedesReceipts).
- Docs: docs/cli.md (`events --since-seq`), CLAUDE.md HQ section, api/contract.md
  untouched (no HTTP surface change).
- Existing specs to reconcile: supervisor-agent, session-events.
- The commander's live HQ home gets migrated on next `gtmux hq` after upgrade.
