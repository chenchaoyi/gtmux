## Why

gtmux only sees agents running **inside tmux** — the radar is built entirely from `tmux list-panes -a`, and the hook keys all state on `$TMUX_PANE`, dropping to a stateless notify when there's no pane. An agent started directly in a terminal tab (no tmux) is therefore **completely invisible**, even though its hooks (Stop / UserPromptSubmit / Notification / SessionStart) still fire and carry `session_id` + `cwd`. We can cheaply **sense** these sessions; users just can't see their live terminal or send input. Surfacing them — and offering a one-click path to bring them under tmux (where gtmux CAN view + control them) — closes the biggest blind spot in the radar.

## What Changes

- The hook, when it fires with **no `$TMUX_PANE`**, records the session by **`session_id`** (agent, cwd, state, last-activity) instead of discarding it — a new `native/` state area, keyed by session, not pane.
- `gtmux agents --json` includes these as rows with **`source: "native"`** (the field already exists). They carry state (working / waiting / idle) and an idle "finished N ago" time derived the same tmux-independent way as tmux rows (`resume.Load` → `transcript.LastMessageTime`), plus agent + project(cwd). They are **not focusable and not send-able** — no terminal locator, no input channel.
- The **menu-bar** and radar surfaces render native sessions in a **separate, clearly-labelled category** ("Elsewhere" / "不在 tmux") so users know they exist and see rough info, without implying they can jump to or reply in them.
- The menu-bar adds an **"Adopt into tmux"** action: select one or more native sessions and, in one click, spawn a fresh tmux session/window that **resumes the same conversation** (`claude --resume <session_id>`, etc.) using the existing resume/restore machinery. Single- and multi-select.
  - **Honest constraint (not BREAKING, but a real limitation):** a running native process cannot be reparented into a tmux pane. "Adopt" re-launches the *same conversation* inside tmux; the original native process is left running. The design must warn about / guide the "two live instances on one conversation" hazard, and only offer Adopt for agents whose sessions are **resumable and whose `session_id` we captured**.
- Detect-only fallback: agents that fire hooks but aren't resumable (or whose id we didn't capture) still appear in the native category as **sense-only** (no Adopt).

## Capabilities

### New Capabilities
- `native-agent-sessions`: sensing agent sessions running outside tmux (hook-driven, session-keyed state; the `source: "native"` radar rows; their reap/staleness lifecycle) and the "Adopt into tmux" action that resumes such a session inside a new tmux session.

### Modified Capabilities
- `agent-radar`: the `agents --json` payload now includes `source: "native"` rows sourced from session-keyed state (not `tmux list-panes`), grouped as a distinct category and marked non-focusable / non-send-able.
- `menu-bar-app`: adds the native-sessions category to the popover and the multi-select "Adopt into tmux" action.

## Impact

- **Contracts:** `gtmux agents --json` schema (additive — `source` already exists; native rows omit tmux-only fields like a focusable locator); new state area `~/.local/share/gtmux/native/<session>` (or similar); the menu-bar `AgentStore` consumer.
- **Code:** `internal/hook/hook.go` (session-keyed path when `$TMUX_PANE` is empty), `internal/app/agents.go` (merge native session rows into the radar), a new native-session store, `internal/resume` + the `restore`/`new` spawn path (reused by Adopt), the macOS app `MenuView`/`AgentStore` (category + Adopt action).
- **Out of scope for now:** live screen/preview of native sessions, sending input to a native session in place, detecting native sessions with **no** gtmux hook installed (we only sense agents that call `gtmux hook`).
