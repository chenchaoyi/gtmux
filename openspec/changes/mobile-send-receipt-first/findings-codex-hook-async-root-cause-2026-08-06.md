# ROOT CAUSE of Finding A — Codex fires no hooks because gtmux installs them ASYNC and Codex 0.146.0 does not support async hooks yet (2026-08-06)

> Evidence appendix to `mobile-send-receipt-first`. Not implemented; captured to survive
> context compaction. This closes the open question left in
> `findings-codex-real-machine-2026-08-06.md` ("why does Codex fire zero hook events?").

## The decisive evidence — Codex's own startup, transcribed from a screenshot

A Codex 0.146.0 session (session `codex-idle-probe`, cwd `/private/tmp`) printed, at
startup, VERBATIM (the ⚠ lines):

```
Tip: Try the Desktop app. Run 'codex app' or visit https://chatgpt.com/codex?app-landing-page=true
⚠ skipping async hook in /Users/ccy/.codex/hooks.json: async hooks are not supported yet
⚠ skipping async hook in /Users/ccy/.codex/hooks.json: async hooks are not supported yet
⚠ clamping SessionEnd hook timeout to 3s in /Users/ccy/.codex/hooks.json
⚠ running async SessionEnd hook synchronously in /Users/ccy/.codex/hooks.json
⚠ skipping async hook in /Users/ccy/.codex/hooks.json: async hooks are not supported yet
⚠ skipping async hook in /Users/ccy/.codex/hooks.json: async hooks are not supported yet
```

(The same screen also showed `MCP client for 'cloudflare-api' failed to start … invalid_grant:
Grant not found`, `MCP startup incomplete (failed: cloudflare-api)`, skill-loading warnings,
and my probe prompt + `PROBE_IDLE_OK` — context, not the essential evidence.)

**Reading:** `~/.codex/hooks.json` has 5 gtmux entries (PermissionRequest, SessionEnd,
SessionStart, Stop, UserPromptSubmit), all `"async": true`. Codex 0.146.0 **SKIPS every
async hook** ("async hooks are not supported yet") — the four `skipping` lines are
PermissionRequest / SessionStart / Stop / UserPromptSubmit; SessionEnd is the one it force-
runs synchronously (clamped to 3s). So `UserPromptSubmit` / `Stop` **never fire**.

## Root cause in gtmux code (this is a gtmux-side choice, not a Codex limitation to wait on)

`internal/app/agent_hooks.go:413-418`, `case formatCodex:` builds each Codex hook entry as:

```go
"type": "command", "command": cmd, "timeout": inst.timeoutFor(b) / 1000, "async": true,
```

The comment (agent_hooks.go:414-415, and :46/:415) is explicit: **`async:true` is
deliberate — "keeps the fire-and-forget `gtmux hook` off the turn's critical path"** so
`gtmux hook` never blocks a Codex turn. But Codex 0.146.0 does not support async hooks yet,
so the flag makes Codex skip them entirely. (`async:true` is also set at
`internal/app/hooks.go:246`.)

## The closed causal chain (all four corroborate, no contradiction)

1. Codex startup: `skipping async hook … async hooks are not supported yet` (screenshot).
2. Live `~/.codex/hooks.json`: all 5 entries `async=true`, `timeout=10`.
3. Code: `agent_hooks.go:417` `formatCodex` hard-codes `"async": true`.
4. Runtime: after submitting several prompts to Codex, `gtmux events` shows **0** events for
   the pane, and the paneless `native/` sink is empty.

⇒ gtmux installs Codex hooks async → this Codex version skips async → **the deterministic
receipt channel is dead** → `Deliver`'s land-verify falls back to a screen-read that races
Codex's >1s composer render → the "NOT delivered" false negatives, the idle-Codex message
loss, and the retry-poisoning (all documented in `findings-codex-real-machine-2026-08-06.md`
and the QA session below).

## The fix (gtmux-side, small — proposal, NOT implemented here)

Install Codex hooks **synchronously** (drop `async: true` for `formatCodex`, or gate it on
Codex version support). Then `UserPromptSubmit` / `Stop` fire, and the receipt channel that
`mobile-send-receipt-first` wants is alive.

**Tradeoff / why it was async, stated plainly:**
- Synchronous ⇒ Codex waits for `gtmux hook` at each event (turn start/end). `gtmux hook` is
  fast (records one event); Codex clamps the timeout to 3s anyway — acceptable.
- Watch the stdin-blocking wedge (cf. opencode #678: a synchronous `gtmux hook` doing
  `io.ReadAll` on an inherited TTY hangs the caller). Codex's native hooks system feeds the
  payload on stdin and closes it, so `io.ReadAll` should return — but this MUST be verified
  with a real sync install + a live Codex before trusting it.

## Related, also unfixed (pointers, so nothing is lost to compaction)

- **New this session — interlock poisons retry:** `gtmux send` returns
  `refused-duplicate` on a retry of a send that FAILED to deliver (the re-send interlock
  keys on "pasted recently", not "delivered"). So a failed Codex send cannot be retried
  without `--force`. Not yet ticketed.
- **QA content-integrity (good news):** a long no-space CJK message, once it LANDS (via the
  working-queue best-effort path), arrives intact — both head and tail markers, no
  truncation, no duplication. Codex send has no #682-class content bug.
- **idle vs working (counter-intuitive):** `gtmux send` to an IDLE Codex loses the message
  (confirmPaste races the slow render → fragment → C-u clears it); to a WORKING Codex it
  lands (pasteBusy best-effort, no C-u → Codex queues it). #697 fixed the working half; the
  idle half is still broken.
- Boundary: no implementation changed, no release; the fix is the commander's call.
