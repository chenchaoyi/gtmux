# HQ startup briefing — self-introduction + immediate status report on spawn

## Why

Today `gtmux hq` spawns the supervisor and drops the user at a bare agent prompt.
The seeded AGENTS.md playbook teaches HQ how to behave, but a fresh session sits
IDLE until the user types the first prompt — so a just-opened HQ tells the user
nothing about itself and nothing about the fleet. The single highest-value moment
(the user just asked for the supervisor) produces a blank screen.

The user wants HQ's FIRST output to (1) say what it is — "我是 gtmux HQ 中控管家" —
with a one-line job description, and (2) immediately deliver a status report (who
needs you, who's working, token usage + subscription room). That turns "I opened
HQ" into "HQ told me where everything stands" with zero extra keystrokes.

## What Changes

On a FRESH `gtmux hq` spawn (not when focusing an already-live supervisor), deliver
a one-shot **startup briefing** prompt into the new pane so the supervisor's first
turn is a self-introduction + fleet status report.

- **Reuses the verified dispatch path** — the same machinery `gtmux spawn` uses:
  `waitAgentReady` (let the agent boot) → `dispatch.Deliver` (paste + land-verify).
  No new delivery machinery, no hand-typed `send-keys` race.
- **Fresh-spawn only.** A `gtmux hq` that finds a live HQ focuses it and returns
  BEFORE the briefing code, so a live session is never re-briefed.
- **Best-effort, non-fatal.** A briefing that can't land never fails `gtmux hq` —
  the session is already up and usable; the user can just type to it.
- **Opt-out.** `GTMUX_HQ_BRIEF=off` (or `0`/`false`/`no`) spawns HQ silently, for a
  user who prefers a quiet start or drives the first prompt themselves. Default on.
- **Bilingual** via `i18n.Tr` (follows `GTMUX_LANG`), so the prompt — and thus HQ's
  response — is in the user's language.
- The briefing's report shape mirrors the seeded playbook's policy #1 (needs-you
  first, ALWAYS include token usage + subscription window), so the prompt stays a
  concise TRIGGER rather than re-specifying the whole format.

## Impact

- Spec: `supervisor-agent` — new requirement "HQ opens with a self-introduction and
  status briefing".
- Code: `internal/app/hq.go` (`cmdHQ` tail; new `deliverHQBriefing` /
  `hqBriefingPrompt` / `hqBriefingEnabled`), reusing `waitAgentReady` /
  `dispatchIO` / `deliverOpts` from `internal/app/dispatchbridge.go`.
- Tests: `internal/app/hq_test.go` (prompt carries both halves in en+zh; opt-out
  toggle parsing).
- No contract change — `agents --json` / `digest --json` schemas untouched. The
  briefing is a delivered prompt, not a new API.
