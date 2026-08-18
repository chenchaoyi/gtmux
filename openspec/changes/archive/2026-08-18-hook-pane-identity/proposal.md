# hook-pane-identity — a hook proves which pane it came from

## Why

On 2026-08-18 pane `%13` went blind for 5h15m and nobody noticed. The phone showed a
calm, complete conversation whose last turn was five hours old; the radar showed a
settled `idle`; HQ's picture of that pane was frozen at 17:45. Nothing errored.

Measured root cause, in two layers:

**Claude Code's side.** At 17:47:52 the session stopped mid-turn, and at 17:48:07 the
pane's `claude` (pid 34175) spawned `claude daemon run --origin transient` → a
`--bg-pty-host` → the real conversation process (`--session-id b63be1ac`). That process
has **no tty and no `$TMUX_PANE`**:

    34175  the pane's interactive client   TMUX=…  TMUX_PANE=%13
    78152  the background session host     (neither)

The conversation — 15 more of the user's instructions, through 22:58 — continued there.
This is an architectural choice of theirs (a session that outlives its terminal), not a
bug, and it will spread.

**gtmux's side, which is ours.** `internal/hook` identifies a pane by exactly one
signal: `$TMUX_PANE`. Absent it, the hook takes the "native (non-tmux) session" branch —
so every event from a backgrounded session was filed under `native/<sessionId>.json` and
pane `%13` received none. The pane→session binding never updated, so `/api/transcript`
kept serving a log that had stopped growing at 17:47.

The identity failure is the root cause, but the reason it cost five hours is that
**every layer stayed silent**. A binding that points at a dead log looks exactly like a
quiet conversation.

## What changes

1. **Identity by evidence, not by one env var.** When `$TMUX_PANE` is absent, walk the
   hook's process ancestry and match it against tmux's own pane table — by `pane_pid`
   first, then by `pane_tty`. Only when nothing matches is the session native. (Measured
   on the failing chain: `78152 → 77301 → 77257 → 34175(ttys014) → 19982`, and tmux
   reports `%13 pane_pid=19982 pane_tty=/dev/ttys014` — both signals hit.)

2. **A stale binding says so.** `gtmux doctor` gains a row that compares each pane's
   bound transcript against the pane's own activity: a pane that has moved while its
   bound log has not is reported, with the pane id and how long it has been silent.

3. **The chat refreshes on its own.** The mobile chat refetches only when the pane's
   status flips or the phone sends a prompt (`DetailScreen.tsx`), so a pane whose status
   is itself frozen can never refresh — the second reason the user saw stale history.
   The view polls while it is open, made cheap by an ETag on `GET /api/transcript`.

## What this does NOT do

- It does not reconstruct the lost events. Those five hours were never observed; deriving
  them from the transcript afterwards would write inference into an audit trail.
- It does not try to keep a background-hosted session *out* of the background. Where the
  agent runs is the agent's business; naming it correctly is ours.
