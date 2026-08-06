# spawn-readiness-persistent-banner — a permanent MCP-auth warning permanently fails the spawn ready gate

> TICKET (regression). No GitHub issues in this repo, so filed as a `changes/` proposal.
> This is a report + fix proposal; it is NOT implemented here. Do not confuse with the
> commander's pending proposals (`mobile-send-receipt-first`, `tmux-id-surface`).

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

## What changes (proposed — not implemented in this ticket)

1. **The ready gate must distinguish "still booting" from "a stable pane that merely carries
   a permanent auth notice".** A persistent `MCP servers need authentication` / `run /mcp`
   line — present on an OTHERWISE-ready, settled composer — must NOT keep the pane
   not-ready forever. (Options for the implementer: drop the auth phrases from `bootBanners`;
   or gate them on the composer NOT being present; or treat a banner that survives the
   two-frame settle as chrome, not boot noise. Design decision left to the fix.)
2. **The timeout evidence must name the blocking line.** When `IsComposerReady` fails at
   timeout, `spawn.go:192`'s message should include the specific boot-banner line that
   `hasBootBanner` matched — so a persistent-banner block is never misread as "agent slow".
3. **Record the workaround** in the eventual fix/docs: `gtmux send --message-file` reaches
   an already-started pane without the ready gate.

## Boundary

Ticket only. No implementation change, no fix PR, no release in this change. Whether/when to
fix is the commander's call.
