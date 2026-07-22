# share-grant-epoch

## Why

Share scopes are stored as tmux pane ids (`%N`), but a pane id is unique only within ONE
tmux server lifetime. After a reboot + restore the ids are reassigned from scratch, so a
stored grant silently comes to address a DIFFERENT pane — a guest can be shown (and given
input to) a session the owner never intended to share, while the session they did share
becomes invisible.

This is not theoretical: on the reporter's machine the stored grants were `%20 %37 %52`,
and after a restore `%20` resolved to an unrelated session (`日常更新:1.0`,
`~/ai_workspace/kb_workspace`) while `%37`/`%52` dangled. The live guest link's `%17`
happened to still land on the intended session — by coincidence, not by design.

## What Changes

Pane grants are bound to the tmux server they were made against, and **refused when that
server is gone** (fail closed) until the owner re-grants:

- `ShareState` gains `pane_epoch`: the identity of the tmux server the grants were made
  against (server pid + process start time — pid alone is reusable).
- The owner granting pane scope (global config, `share new`, `share set`) **stamps** the
  running server's identity.
- Every guest pane check refuses when the stamp doesn't match the running server: attach
  (before any PTY is spawned), `/api/send`, pane content, and the guest radar (which
  returns an empty list rather than risk revealing an unshared pane).
- The capability (`GET /api/share`) and `gtmux share status` report `stale`, and the CLI
  prints a loud warning telling the owner to re-grant — otherwise sharing appears
  silently broken.
- Unstamped legacy grants are treated as stale (correct: they predate the running server).
- With no tmux server / unreadable identity, nothing is considered stale — this can never
  lock out a working setup.

Deliberately NOT auto-remapping by session name: names repeat, get renamed, and can be
taken over by another project, so an automatic remap could re-grant the wrong thing —
which is the very failure being fixed.

## Impact

- Spec: `remote-access` (guest scope validity).
- Code: `internal/server/share.go` (epoch state, `GrantsStale`, `StampEpoch`, capability),
  `internal/server/attach.go` + `server.go` (the four guest gates),
  `internal/app/serve.go` (`tmuxServerEpoch`), `internal/app/sharecmd.go` (status + warning).
- Back-compat: additive JSON field. Existing grants become stale once (by design — they
  are genuinely unverifiable) and need one re-grant.
