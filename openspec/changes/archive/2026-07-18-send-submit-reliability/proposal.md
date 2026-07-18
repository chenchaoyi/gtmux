# Change: send-submit-reliability

## Why

Delivering long / multi-line text into a coding-agent composer is unreliable in
practice: the paste and the follow-up Enter race, so a task is (1) **truncated** —
only the first part submits and the tail is cut off, or (2) **not submitted at
all** — even a bare `--key Enter` fails to land (reproduced repeatedly: a task
tail severed, and a `.m2` instruction to pane `%83` where two manual Enters never
took). Root cause: the "paste landed" check confirms only the first **40-rune
head** of the payload (`ContainsHead`) before firing Enter, so a half-rendered
draft gets submitted and then reported as `landed`; and `paste-buffer -p`'s
bracketing is *conditional* on the app's mode, so a paste that arrives while
bracketed-paste is off streams raw newlines (each a premature submit) and can
leave an unterminated paste state that swallows the subsequent Enter.

The existing `agent-dispatch` spec already says the system SHALL "confirm the FULL
task text (not a prefix)" before submitting — the code just doesn't honor it. This
change makes the code match the spec and closes the gap on the paths that skip
verification entirely.

## What Changes

- **Full-payload paste confirmation (not head-only).** The pre-submit draft check
  requires the payload's **head AND tail** fingerprint (or a collapsed-paste
  placeholder) to be present before Enter is sent — a 40-rune prefix is no longer
  sufficient. A settled head-without-tail is a fragment (retry), never a submit.
- **Atomic bracketed delivery.** The paste is delivered so that a multi-line
  payload can never submit line-by-line and never leaves an unterminated
  bracketed-paste state that eats a later Enter — the delivery confirms the draft
  before submitting rather than trusting `-p` unconditionally.
- **Pre-submit content check on the unverified paths.** `POST /api/send` (phone /
  menu-bar reply) and `gtmux send --no-verify` gain the cheap "draft holds the
  full text, THEN Enter" step. They still skip the post-submit *landed*
  verification (to stay fast on the phone's latency budget) but no longer paste
  and blindly Enter into a racing composer.
- **Don't blindly re-Enter.** `Deliver`'s swallowed-Enter retry re-confirms the
  draft still holds the **full** target before re-sending Enter; if the draft is
  empty (already submitted) or no longer matches, it does not fire another Enter.

## Capabilities

### Modified Capabilities

- `agent-dispatch` — the delivery/verification requirements are tightened:
  full-payload (head+tail) pre-submit confirmation, atomic bracketed delivery with
  no lingering paste state, a content check before every Enter (including
  re-submit), and the unverified paths (`POST /api/send`, `--no-verify`) confirm
  the draft before submitting.

## Non-goals

- No change to the post-submit *landed* verification budget or the hook-first /
  screen-fallback layering — the unverified paths stay unverified for landing.
- No new CLI flags or HTTP surface; the wire contracts are unchanged.
- Not agent-specific: the fix is bracketed-paste + composer content comparison,
  so it works for any agent TUI, not only Claude Code.
