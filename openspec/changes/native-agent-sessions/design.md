## Context

The radar reads only `tmux list-panes -a` (`internal/app/agents.go`), and the hook keys every state marker on `$TMUX_PANE` (`internal/hook/hook.go`), explicitly degrading to a stateless notify when there's no pane. So agents run outside tmux (`claude` in a bare terminal tab) are invisible.

But the hook itself is tmux-independent: Claude/Codex/etc. call `gtmux hook --agent <k> <event>` on Stop / UserPromptSubmit / Notification / SessionStart regardless of tmux, passing `session_id` + `cwd` on stdin. The idle-duration machinery is already session-keyed and tmux-free (`resume.Load(loc)` → `transcript.LastMessageTime(agent, sessionId)`; see the idle-since design). So we can sense a session's existence + state + last-activity without a pane — we just have no terminal to view or a channel to type into.

## Goals / Non-Goals

**Goals:**
- Sense agent sessions that fire gtmux hooks but run outside tmux; surface them as a distinct, clearly-limited category in the radar + menu bar.
- Reuse the existing session-keyed idle-time + resume machinery — no new time source.
- Offer a one-click "Adopt into tmux" that resumes such a conversation inside a fresh tmux session, single- or multi-select.
- Keep `gtmux agents --json` backward compatible (additive; `source` already exists).

**Non-Goals:**
- Live screen/preview of a native session, or sending input to one in place (there is no terminal locator and no input channel).
- Detecting agents that don't call `gtmux hook` (no hook = no signal).
- Actually reparenting a running process's PTY into tmux (impossible).
- Per-terminal native detection via tab-title scanning (that's the separate "multi-terminal" deferred scope).

## Decisions

### 1. Session-keyed native state, written by the hook
When `$TMUX_PANE` is empty, instead of dropping state the hook writes a per-session record under a new state area, e.g. `~/.local/share/gtmux/native/<session_id>.json` = `{agent, sessionId, cwd, state, updatedAt}`. `state` is derived from the same `decide()` lifecycle the tmux path uses (UserPromptSubmit→working, Stop→idle, waiting events→waiting), but keyed by `session_id`. This keeps ONE state machine; only the key differs (session vs pane).

Rationale: the hook already has `session_id` + `cwd`; keying by session is the minimal change and matches how resume/transcript already work.

### 2. Radar merges native rows (`source: "native"`)
`internal/app/agents.go` reads the `native/` store and emits rows with `source: "native"`, omitting tmux-only fields (no focusable `loc`/pane). Idle "finished N ago" reuses `transcript.LastMessageTime(agent, sessionId)`. A native row whose `session_id` ALSO appears as a live tmux pane is suppressed (it got adopted / is really in tmux) — de-dupe by session id, tmux wins.

### 3. Separate category + limited affordances
Both the menu-bar popover and any radar surface group native rows under their own header ("Elsewhere" / "不在 tmux"), visually marked as sense-only: no jump chevron, no reply/send, no focus on click. The row shows agent, project (cwd basename), state, idle time, and (optional) last prompt.

### 4. "Adopt into tmux" = resume, not move
Adopt spawns a fresh tmux session/window (reuse the `new` + `restore` spawn path) running the agent's resume command from `internal/resume` (`claude --resume <session_id>`, the same argv `restore` uses). It does NOT touch the original process. Multi-select adopts each into its own window/session.

- Only offered when the agent is **resumable** (`resume.Resumable`) and we captured a `session_id`; otherwise the row is detect-only (no Adopt button).
- **Two-instances hazard:** after adopt, both the original native process and the new tmux one point at the same conversation log. The design warns in the Adopt confirmation ("close the original terminal — the resumed session takes over") and, on adopt, the native record is marked "adopted" so it drops out of the native category (the tmux pane now represents it). We do NOT try to kill the original process (we can't reliably identify/own it, and killing another terminal's process is out of scope + risky).

### 5. Reaping / staleness
Native records have no pane whose disappearance signals "gone." A `SessionEnd` hook removes the record. Absent that, a record is treated as stale after a grace window past `updatedAt` (e.g. hours) so a long-dead native session doesn't linger forever. Idle native sessions legitimately persist (like idle tmux ones), so the grace is generous; the reap targets records we simply stop hearing about. (Open question below on exact policy.)

## Risks / Trade-offs

- **Adopt creates a duplicate.** Mitigated by the warning + marking the record adopted; not by killing the original (out of scope). Accept that a careless user could drive two instances briefly.
- **No hook = no visibility.** A native agent without gtmux hooks installed stays invisible. Acceptable — sensing requires the hook, and install-hooks is the existing onboarding.
- **Session-id de-dupe.** A resumed-in-tmux session must suppress its native twin; relies on the tmux side exposing the same `session_id` (via the resume record) so we can match. If matching is imperfect a row could briefly double-show. Low harm.
- **State-machine reuse must stay single-source.** Splitting the hook into pane-keyed vs session-keyed risks divergence; keep `decide()` shared and only branch the key + store.
- **Reap policy is a judgement call** (see open question) — too aggressive hides real idle sessions; too lax clutters with dead ones.

## Open Questions
- Reap policy: fixed grace past `updatedAt`, or only reap on explicit `SessionEnd`? Leaning: reap on `SessionEnd` + a generous fallback grace.
- Should Adopt attempt to signal/close the original process at all, or purely guide the user? Leaning: guide-only for v1.
- Do we show a native row's last prompt (needs a transcript read) or keep it to agent/project/state/time to stay cheap? Leaning: cheap first, last-prompt as a follow-up.
