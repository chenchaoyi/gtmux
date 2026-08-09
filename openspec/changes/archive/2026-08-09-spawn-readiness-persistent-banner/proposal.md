# spawn-readiness-persistent-banner — a permanent MCP-auth warning permanently fails the spawn ready gate, and `gtmux tasks` calls the wreckage `done`

> Filed as a `changes/` proposal (no GitHub issues in this repo). Started life as a
> report; the commander approved implementation on 2026-08-09 and widened it with the
> ledger defect (§4) — so this document is now the CHANGE, not a ticket.

## Why

On 2026-08-06 (v0.45.6), `gtmux spawn --title draft-fp-repro --goal-file <file>` started a
session (`%33`) but the goal was **never delivered**:

```
✗ NOT delivered → draft-fp-repro:0.0 (%33) · ✳ Claude Code — evidence:
agent composer not ready within the ready timeout
…
 ⚠ 10 MCP servers need authentication · run /mcp
❯                       ← composer empty, goal never entered
```

Reproduced verbatim on a re-run (the charter's "re-run a failed spawn" correctly resumed
`%33` — "goal was never delivered" — but the ready gate still did not pass). Two facts were
verified this morning and are recorded so the next person does not re-investigate:

- **Workaround works:** delivering the SAME goal to the already-started pane with
  `gtmux send --message-file <path>` succeeded first try. (`send` does NOT call
  `IsComposerReady` — a different, non-gated path.)
- **The timeout message is unreadable:** it says only `agent composer not ready within the
  ready timeout` and never names WHICH line blocked the gate. This is why this footgun is
  repeatedly misread as "the agent is slow".

It recurred twice more on 2026-08-09 (`%68`, `%69` — both goals landed only via the
`gtmux send --message-file` rescue), which is what promoted the ticket to a change.

**This is a REGRESSION, not a new footgun.** KB `pitfalls.md` records the root cause
(2026-08-01, found on `%29` doing `--goal-file`): the readiness gate treats this Mac's
permanent `N MCP servers need authentication` line as *startup noise*, so the gate never
settles and always times out into `NOT delivered` — reporting a timeout, never the banner.
KB further records it as **"✅ fixed 2026-08-01, PR #652 / v0.44.10: bootBanner matching
narrowed; persistent warnings no longer treated as startup noise; on ≥0.44.10 `NOT
delivered` is a trustworthy signal."** On 0.45.6 the same banner, same timeout, same empty
composer — it is back.

**Root cause of the current failure (code-visible, low-cost — not a guess):**
`internal/prompt/prompt.go:160-168` STILL lists `"MCP servers need authentication"` and
`"need authentication"` as boot banners, and `hasBootBanner` (`prompt.go:196`) matches a
multi-word signature anywhere on the bottom 14 lines via `strings.Contains`. This Mac's
permanent `10 MCP servers need authentication · run /mcp` sits in that bottom region → the
pane is judged still-booting forever → the ready gate (`internal/dispatchbridge`,
`prompt.IsComposerReady`) never settles → `spawn.go:192` reports `NOT delivered`.

**Why #652 did not cover this form (read from the code, not guessed):** #652's narrowing
(`prompt.go:175-186`) changed a WHOLE-capture `strings.Contains` to a BOTTOM-region scan +
line-anchoring for one-word spinners — it fixed the "transcript merely MENTIONS
loading/connecting" false-positive. It did **not** remove the MCP-auth phrase, and
multi-word signatures still match anywhere on a bottom line. So a *genuinely persistent
bottom-line auth warning* — as opposed to a transcript mention — was never addressed. From
the user's side it is a recurrence; in the code it is an incomplete fix + an environment
that now carries the banner permanently.

### The SECOND defect: `gtmux tasks` reports a never-delivered dispatch as `done`

Fixing the gate does not fix this, and it is what made the failure *invisible* rather than
merely annoying. `taskStatus()` (`internal/hq/taskscmd.go:41-47`) derives a tracked
dispatch's status from ONE thing — the pane's live radar status — and
`radar.TaskStatusFor` maps `idle → "done"` (`internal/radar/digest.go:208-216`). A
ready-timeout leaves behind exactly a live, empty, **idle** agent pane, so the ledger row
renders as green `✳ done` (`taskscmd.go:244-245`) with the full `--goal-file` text in the
Goal column — and that Goal is the *intent* spawn recorded (`spawn.go:195`), never the
delivery result.

The ledger already knows better: `spawn.go:196` writes `Delivered: false`
(`internal/dispatch/ledger.go:30`) on that very path. **Nothing but `ledger.go:172`'s
resume lookup ever reads it.** So the one durable record of "this never landed" is
excluded from every place a human or HQ looks.

Verified on this machine while writing this change — all three of 2026-08-09's dispatches
are on disk as `delivered: false` (`%66` deliver-v0480, `%68` check-signal-ergo, `%69`
impl-spawn-readiness), and all three are working normally because a human re-sent their
goals by hand.

### Resolved: was the failure silently swallowed, or just drowned?

The open question was whether HQ's two spawns took a failure path that prints nothing
(which would be its own bug). **It does not exist — the failure IS reported**, and the
evidence is threefold:

1. Every non-landed exit in `spawn` funnels into `spawnReport`, whose `default` branch
   prints `✗ NOT delivered → <handle> — evidence:` on **stderr** and returns 1
   (`spawn.go:678,685`). There is no `return 0` and no silent branch: `landed` prints ✓,
   `queued` prints `•`, `refused-duplicate` prints `✗ refused`, everything else prints
   `✗ NOT delivered`.
2. The only way to get a `delivered:false, own_session:true` ledger entry — which is what
   all three of today's entries are — is the ready-timeout branch (`spawn.go:191-198`),
   whose sole exit is `spawnFail → spawnReport`.
3. So the line was emitted and drowned: `res.Evidence` appends `tmux.CaptureFull(pane)`,
   **measured at 224 lines / 11.8 KB** of live agent TUI on this machine. The one-line
   verdict is the head of a full-screen wall that reads exactly like a normal startup log
   — which is precisely how it was described ("spawn 不报错,还回显新 pane 的整屏").

That is not "the operator should read more carefully" — it is a reporting defect, and it
is fixed here as part of §2: name the blocker in the first line, and cut the evidence to
the bottom region (the convention `dispatch` already uses for its own failures via
`evidenceTail`) instead of dumping the scrollback.

## What changes

1. **A standing notice must stop failing the ready gate forever.** The two authentication
   phrases leave `bootBanners`; a persistent `N MCP servers need authentication · run /mcp`
   line on an otherwise-ready, settled composer reads as READY. A genuinely still-booting
   pane (connect/load/init banner, or no composer row yet) is still caught.
2. **The ready-timeout evidence names the blocker.** `spawn`'s failure evidence leads with
   the specific line/condition that kept the pane not-ready, and carries the bottom region
   of the capture rather than 200 lines of scrollback.
3. **Consistency** — TROUBLESHOOTING entry, KB correction, spec sync, archive.
4. **`gtmux tasks` (and the digest's `task_status`) SHALL respect the delivery record.** A
   ledger entry whose goal never landed reads `undelivered`, never `done`, regardless of
   what the pane is doing. The ledger stores the dispatch `state` alongside `delivered` so
   the distinction survives a restart, and a rescue `gtmux send` that LANDS back-fills the
   entry — so the honest workaround stops leaving a permanently-wrong row behind.

### The mechanism for §1, and why the other two candidates are not real alternatives

The earlier ticket left three candidates open (drop the phrases / gate them on the composer
being absent / treat a banner surviving the settle as chrome). Evaluating them against the
stated boundary — *the narrowing must not eat genuine startup noise* — collapses two of
them:

- **(B) "match the auth phrases only when no composer prompt is present"** is a no-op that
  reads as a fix. `IsComposerReady = hasPromptLine && !gate && !banner`. Gating the banner
  check on `!hasPromptLine` makes `!banner` vacuously true in every case the conjunction
  can reach — i.e. it silently DELETES the banner check for all signatures, not just the
  auth ones. And the check exists precisely because a composer row CAN be on screen while
  the agent is still booting: if it could not, `hasPromptLine` alone would always have
  sufficed and `bootBanners` would never have been written.
- **(C) "a banner that survives the two-frame settle is chrome"** collapses into (B). The
  settle requirement is already *two byte-identical captures*; any banner present on a
  settled pane survives it by definition. So (C) means "the banner only counts on
  unsettled frames" — the same nullification, bought with a rewrite of a pure string
  predicate into cross-frame state.
- **(A) recategorize** is the only one that narrows rather than deletes, and it is also the
  semantically correct call: `⚠ N MCP servers need authentication · run /mcp` is not boot
  *noise*, it is a boot *result* naming an action only the user can take. Boot noise is
  what resolves by waiting; this never does. The transient sibling — `Connecting…` while
  MCP servers are still coming up — stays in `bootBanners` and still holds the gate, so the
  real hazard the list was written for (a long paste into a repainting TUI) is unchanged.
  The residual window (a pane that shows the auth line while still repainting) is covered
  by the settle check, which is the mechanism designed for exactly that.

## Boundary

- §1 must not weaken the gate against a genuinely booting pane — pinned by a control test,
  not by inspection.
- `undelivered` is an ADDITIVE value in the `gtmux tasks --json` `status` enum and the
  digest's `task_status`; no consumer outside the Go CLI reads either today (checked:
  macapp and mobileapp reference neither), so no surface breaks.
- Out of scope: authenticating the MCP servers (`/mcp`) — a user action, not gtmux's; and
  the unverified `POST /api/send` path, which has no landing verdict to back-fill with.
