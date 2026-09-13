# Research notes: what to borrow from cmux + multiplexer (mux) adapters

> 2026-06 survey. Based on the cmux source (`manaflow-ai/cmux`, read-only).
> Two goals: ① find the high-value pieces worth feeding back into gtmux; ② assess
> supporting other mainstream "muxes" through an **extension/adapter** layer as a
> side benefit (a bigger user base), with gtmux staying native-tmux first.

## TL;DR

Borrow cmux's **intelligence**, not its **plumbing**. The primitives — enumerate /
read screen / send / focus — come free with gtmux's tmux foundation, and they are
exactly where cmux, going through its socket, is **weaker** (see the two gaps in §2
below). What is genuinely valuable is how cmux handles "judge the agent's state, name
things, resume per agent" — which is currently gtmux's thinnest layer.

---

## 1. High-value borrows (ranked by ROI)

### ⭐ A. Precise "waiting on you" detection: `FeedEventClassifier` (highest value)
cmux does not scrape titles or run timers. It uses a **typed `(source, event) →
meaning` registry** to decide, straight from the hook payload, whether an agent is
**blocked on you**, and it distinguishes **permission request / plan approval
(ExitPlanMode) / question (AskUserQuestion)**. The correctness details that matter:
- For agents **with dedicated approval events** (claude/codex/hermes), "tool started"
  is always telemetry, never mistaken for an approval (their issue #4985).
- For agents **without dedicated approval events** (gemini/kiro), an event is only
  promoted to actionable when the tool is **side-effecting** (Bash/Write/Edit/
  apply_patch/shell/… see `sideEffectingTools`).
- Evidence: `CLI/FeedEventClassifier.swift` (~340 lines of pure logic, no app
  dependency, **portable to Go almost line by line**).

gtmux currently approximates this with "pane title braille/✳ + event timing"; with
this deterministic classifier, waiting detection goes from "guessing" to "fact".
**Effort: medium (1–2 days to port + the side-effecting tool set).**

### ⭐ B. LLM auto-naming: pane name = a 2–5 word task summary (highest ROI)
cmux's tab name is not the pane title; it is a short title produced by **summarizing
the agent conversation with a small model**.
- Trigger: the Stop/turn-end hook, **throttled** (≥12 transcript lines, ≥6 lines of
  growth since last time, ≥180s apart).
- Parses several transcript formats: Claude Code JSONL, Codex rollout, Grok
  `chat_history.jsonl`, and cached hook payloads for opencode/pi/omp.
- Summarizing: `claude -p --model haiku --tools "" --no-session-persistence` (or
  `codex exec` in a read-only sandbox); the prompt asks for "2–5 words, same language
  as the conversation, output only the title", and it **clears `CMUX_*`/
  `CLAUDE_CODE_*` before the call so the hook cannot fire recursively**.
- Evidence: `CLI/CMUXCLI+AutoNaming.swift`, `+AutoNamingDispatch.swift`.

gtmux's task column is currently the pane title; with this wired in, the task
description becomes meaningful and follows the conversation's language. Pure logic,
no socket needed. **Effort: medium.**

### ⭐ C. Declarative multi-agent hook registry (support surface from 1 → 17)
gtmux currently integrates only Claude Code. cmux covers **17 agents** (codex/gemini/
cursor/opencode/grok/amp/kiro/hermes/copilot/…) with one `AgentHookDef` array; adding
a new agent ≈ appending ~15 lines of data. The generated hook commands are hardened:
auto-locate the CLI, gate on `$CMUX_SURFACE_ID`, one disable switch per agent, and an
`|| echo '{}'` fallback (a hook can never break the agent).
- Evidence: `CLI/CMUXCLI+AgentHookDefinitions.swift` (`agentDefs` 156–371).

With this structure, gtmux's "multi-agent support" becomes filling in a table.
**Effort: medium** (the bulk is each vendor's config format — flat/nested/yaml/plugin;
do `.flat` + `.nested` first).

### D. Resume the **session** per agent (not a dead pane)
tmux-resurrect restores shells, not agent conversations. cmux records each agent's
launch method + sessionId and rebuilds `<agent> --resume <id>`: claude `--resume`,
codex `resume <id>`, amp `threads continue`, opencode `--session`… (18 vendors in all).
- Evidence: `Packages/macOS/CMUXAgentLaunch/.../AgentResumeArgv.swift` (`builtInKind`).
- Supporting pieces: the snapshot stores `wasAgentRunning` + (ANSI-safe truncated)
  scrollback to decide **which** panes auto-resume; `autoResume` is opted in per pane,
  with a signed, tamper-proof approval record.

This is the biggest resilience upgrade over tmux-resurrect. **Effort: medium** (pure
table lookup; the hard part is capturing each vendor's sessionId in the spawn wrapper).

### E. Two pitfall guards in the hook session store
Once gtmux goes deep on hooks it will hit these two traps; cmux has already solved them:
1. **A late hook from an old session in the same pane writes the wrong state** →
   blocked by the "current session boundary" in `activeSessionsBySurface` (issue #5908).
2. **A sub-agent's Stop marks the still-working main agent idle** → handled with the
   nested turn stack `activePromptTurnIds` (`recordPromptSubmit`/`recordPromptStop`).
- Evidence: `ClaudeHookSessionStore` in `cmux.swift` (flock-locked, 7-day cleanup).
  **Effort: medium.**

### F. Odds and ends from the mobile / remote layer
- **Silent "dismiss / badge sync" pushes**: priority-5 `content-available` +
  `apns-collapse-id` coalescing + absolute badge values + **fan-out to every device**
  (a second, offline phone also gets cleared). gtmux already has an APNs relay; adding
  this message discipline is a clean UX upgrade. `Sources/Cloud/PhonePushClient.swift`,
  `web/services/apns/sender.ts`. **Effort: low.**
- **`hideContent` + "push only when away"**: terminal content never leaves the Mac; no
  pushes while the Mac is in use. **Low.**
- **Credentials only over encrypted routes**: the token may only travel over Tailscale/
  loopback; plaintext LAN is refused with a warning (`MobileShellRouteAuthPolicy.swift`).
  Directly applicable to gtmux's tunnel/LAN story. **Low.**
- **The pairing code is not itself a credential** (advanced): cmux's pairing QR carries
  only `host:port` + an opaque user id; the real auth is the account token the phone
  already holds — safer than gtmux's `{url,token}` (a screenshot leaks it), and the
  pairing code need not expire. Needs an identity layer, though; a lightweight version:
  make the pairing token merely "a credential to exchange for a short-lived one".
  **Effort: medium–high.**
- **Structured render-grid streaming** (full-screen snapshot + row-level diff + style
  table + scrollback + `state_seq` resync) uses less bandwidth than gtmux's current
  capture-pane-over-SSE and reconnects more robustly. But it is a protocol + a client
  emulator, **effort: high**; the status quo is adequate, so file it as an optimization.

---

## 2. Multi-mux adapters: architecture and reality

### gtmux detection is really two halves
- **(a) Enumerate + read titles/commands**: `tmux list-panes -F …` — this half is
  **tightly coupled to the mux**; every mux needs an adapter.
- **(b) The `⏸ waiting` / `✓ latest` hooks**: state is keyed by **each pane's id**.
  tmux gives `$TMUX_PANE`, cmux gives `$CMUX_SURFACE_ID`; same thing in essence. **Abstract
  the "pane-id source" and this half is almost mux-agnostic by nature.**

So "support other muxes" = define a `Multiplexer` interface (list / read screen / send
/ focus / new) + parameterize the hook's pane-id source. tmux is reference
implementation #1.

### cmux as an adapter: doable, but the radar gets weaker
cmux's socket exposes enough for a **read-only adapter**: `surface.list` (enumerate +
title + tty), `surface.read_text` (read screen, **plain text, no ANSI**),
`surface.send_text/send_key` (send), `surface.focus` (focus); every pane has a
`CMUX_SURFACE_ID`, and auth is `cmuxOnly` (process lineage) or a password.

**But there are two real gaps (and they matter most for a "radar")**:
1. **Per-pane live state (busy/idle/exited) is not queryable over the socket** —
   internally it is pushed into cmux via `report_shell_state`, but no read RPC returns
   it. tmux gives `#{pane_current_command}` / `pane_dead` directly. **This is the radar's
   biggest gap.**
2. **The live foreground command + PID are not on the surface row** — you must call
   `system.top` separately and join on tty, which is heavier and racier than a single
   tmux `list-panes -F`.

> Implication: even with a cmux adapter, the radar's "state" would have to come from
> **gtmux installing its own hooks into cmux panes** (keyed by `CMUX_SURFACE_ID`), not
> from cmux's socket. Which confirms "borrow the intelligence (hook classification),
> don't depend on its plumbing".

### Other muxes worth including in the interface design
- **Zellij** (Rust, the most popular non-tmux mux): has CLI/action + plugins; adapter #3.
- **WezTerm multiplexing**: `wezterm cli list` (JSON) enumerates + `send-text`;
  adapter-friendly.
- **tmate / byobu**: tmux underneath; already work today.
- **screen**: enumerable but state-poor; low priority.

When designing the `Multiplexer` interface, align the capability surface across tmux +
cmux + Zellij + WezTerm, and fill the gaps (e.g. cmux's live state) uniformly with
"gtmux's own hooks".

---

## 3. Suggested order

1. **Pure-logic borrows first (mux-independent, immediate UX gain)**: A waiting
   classifier → B auto-naming → C multi-agent hook registry. These three lift gtmux's
   experience on tmux a full notch and lay the groundwork for multi-mux (hook
   classification is mux-agnostic).
2. **Then extract the `Multiplexer` adapter interface** + parameterize the hook's
   pane-id source; tmux becomes the reference implementation.
3. **Add adapters last, on demand** (WezTerm/Zellij make better "radar adapters" than
   cmux because they can report live state; a cmux adapter needs gtmux's own hooks).
4. Pick up the F-series mobile/push odds and ends as you go (low-cost increments).

> Strategic reminder: cmux already has a sidebar radar + notifications + waiting
> detection + session resume + **a paired-phone iOS app**. For someone **already on
> cmux**, gtmux adds little. The point of multi-mux adapters is to **cover WezTerm/
> Zellij/remote tmux** — the scenarios cmux cannot reach — not to poach users on cmux's
> home turf.
