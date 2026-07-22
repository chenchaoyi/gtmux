# resume-record-ownership

## Why

A pane's resume record answers one question: if this pane came back empty, which
conversation should be relaunched into it? It is keyed by the pane's tmux locator, and
until now ANY hook firing with that `$TMUX_PANE` overwrote it — "a different session id"
was taken to mean "the pane changed hands".

That is wrong, because a coding agent runs some of its own machinery as SEPARATE sessions
in the SAME pane. Claude Code runs a slash command like `/usage` as its own session id,
in the same pane, firing the same hooks — and it takes the pane's record from the
conversation actually running there.

Observed live on the reporter's machine while verifying an unrelated fix:

```
resume/gtmux:0.0  → 1dad3897…   log 3.5 KB   (a /usage stub: one command envelope)
the real session  → affb005b…   log 72 MB    (the conversation in that pane)
```

Two consequences, the second much worse than the first:

1. **The chat view goes blank** on a pane mid-conversation — it resolves the pane's
   history through this record, so it reads the stub and shows "no conversation history".
   That is how the bug was noticed.
2. **`restore` would relaunch the stub.** After a reboot it runs
   `claude --resume <recorded id>` — a `/usage` screen in place of the work session, with
   the real conversation unreachable from the very record meant to protect it. This is
   the same failure mode as the restore data-loss already fixed this cycle, arriving
   through a different door.

## What Changes

A session may take a pane it does not already own **only if it is a real conversation** —
something the transcript parser can turn into at least one turn. A command stub has no
turns, so there is nothing to resume into and no reason to hand it the pane.

- No incumbent, or the same session reporting again → recorded, as today (the common
  path, decided without reading any log).
- A different session with a parseable conversation → takes over. A genuine handover
  (you quit the agent and start fresh in the pane) still works; the guard is about stubs,
  not about newness.
- A different session with no conversation → refused, **unless** the incumbent's log is
  gone. A record pointing at a conversation that no longer exists can never be resumed
  and must not pin the pane forever.

"Is there a conversation here" is answered by the same parser the chat view uses, so the
question has exactly one definition in the product.

## Impact

- Specs: `session-restore` (which conversation owns a pane).
- Code: `internal/hook` (the only writer of resume records).
- Not changed: the record schema, the locator key, `restore`'s matching, or the
  same-session update path — a correct record is written exactly as before.
