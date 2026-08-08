# ROOT CAUSE of Finding A — Codex fires no hooks because gtmux installs them ASYNC and Codex 0.146.0 does not support async hooks yet (2026-08-06)

> Evidence appendix to `mobile-send-receipt-first`. This closes the open question left in
> `findings-codex-real-machine-2026-08-06.md` ("why does Codex fire zero hook events?").
>
> **STATUS — the root cause below is FIXED (PR #703, merged 2026-08-06).** This note is now
> the EVIDENCE RECORD, not an open ticket. #703 landed all three parts the analysis called
> for: codex hooks install SYNC (no `async:true`), a small `codexHookTimeoutSec` = 3s, and
> `hook.Run` re-execs itself detached (`setsid`, codex-only) so the tmux captures/nudges
> never block Codex's turn. Live-confirmed on this Mac: a fresh Codex session now fires
> `SessionStart`/`UserPromptSubmit`/`Stop` into `gtmux events` (it fired **0** before).
> Existing installs need `gtmux install hooks --agent codex`; Codex re-prompts to TRUST the
> changed `hooks.json` (press `t`). The "Related, also unfixed" items at the bottom are
> **NOT** covered by #703 and remain open.

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

## Documentation-confirmed (official Codex docs, web research 2026-08-06)

Cross-checked against `learn.chatgpt.com/docs/hooks`, the config-reference, and openai/codex
PRs. Confirms the root cause AND surfaces a cost the async choice was hiding:

- **`async` is documented-but-unimplemented.** Doc, verbatim: *"The `async` option is parsed,
  but asynchronous command hooks aren't supported yet."* Codex **skips** `async:true` handlers
  for every event **except `SessionEnd`** (run synchronously with a warning; default timeout 1s,
  hard-capped 3s — PR #33895). Exactly our 0.146.0 log.
- **Fix confirmed: install SYNC** — omit `async` or set `async:false`; **the default when
  absent is `false` (synchronous)**. `async:true` is the ONE value Codex skips.
- **⚠ Cost the async flag was hiding — a SYNC command hook BLOCKS the turn** until it returns
  or hits `timeout` (default **600s** for most events; Codex awaits matched handlers via
  `join_all`). So "drop async" is not enough on its own: **`gtmux hook` MUST return fast**
  (record the event, detach/fork any slow work) **and the codex entry's `timeout` should be
  SMALL** (the live one is 10s — shrink it), or every `UserPromptSubmit`/`Stop` adds latency /
  a hang risk to Codex's turn.
- **VERIFIED (2026-08-06): `gtmux hook` is NOT instant and does NOT detach.** Synchronously it
  runs a `ps` subprocess (process-ancestry, `hook.go:169`) + several `tmux` calls (`Display`,
  `Attended`, `list-panes -a`), and the **Stop/done path does TWO full captures**
  (`tmux.CaptureFull` + `tmux.CaptureFullColor`, `nudge.go:184/192`) + `DraftOfColored` — no
  goroutine/process detach; it returns only after all of it. Typically tens–hundreds of ms
  (heaviest on Stop), with a wedge-stall risk (a stuck `ps`/tmux once froze the radar). So a
  clean sync fix is THREE parts, not one: (1) drop `async:true`; (2) small codex `timeout`
  (2–3s); (3) **make the codex hook path record-and-DETACH** (return immediately, do the
  captures/nudges in a backgrounded process) — otherwise every Codex turn start/end eats that
  latency and inherits the wedge risk. Minimal-viable = (1)+(2), accepting the per-event delay.
  *(Superseded by #703, which took all three — including the detach; the paragraph above
  describes the pre-#703 code.)*
- **`notify` is NOT a substitute** for the receipt: it fires only `agent-turn-complete`
  (turn-end), never prompt-submit; docs say *"New integrations should use the hooks.json
  system."* The `UserPromptSubmit` hook delivers the **prompt text** (stdin JSON `"prompt"`) and
  `Stop` delivers `last_assistant_message` — both are what a receipt / land-verify needs.
- **No Codex release has landed async support** as of 0.146.0 (docs still mark it reserved).
- Sources: https://learn.chatgpt.com/docs/hooks · https://github.com/openai/codex/pull/33895 ·
  https://learn.chatgpt.com/docs/config-file/config-reference

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
- Boundary: this note changed no implementation. The hook-async root cause it identified was
  then fixed by **#703** (see the STATUS block at the top); everything in THIS section is
  outside that fix and still open.
