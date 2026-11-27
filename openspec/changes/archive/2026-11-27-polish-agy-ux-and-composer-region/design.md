# Design: Antigravity (`agy`) UX, Session Titles, Profile Metadata, and Multi-Box Composer Detection

## 1. Multi-Box Composer Detection (`internal/dispatch/region.go`)

When `agy` has in-flight background tasks (`1 task(s)`), its TUI paints two stacked horizontal boxes at the bottom of the screen:

```text
──────────────────────────────────────────────────────────────────────  ← prevTop (Border 1)
>                                                                       ← composer (prompt line)
──────────────────────────────────────────────────────────────────────  ← top     (Border 2)
 ● [19:56:02] cd /path/to/repo && ...                          running  ← auxiliary task drawer
──────────────────────────────────────────────────────────────────────  ← bottom  (Border 3)
 Press up to edit queued messages      auto · Gemini 4 Carbon · 1 task(s)
```

`SplitInputRegion` currently picks `(top, bottom)` = `(Border 2, Border 3)`. Because `lines[top+1:bottom]` has no prompt marker (`> ` / `❯ `), we inspect whether an upper border `prevTop` exists within a bounded distance (`<= 32` lines) above `top` such that `lines[prevTop+1:top]` begins with a recognized prompt line (`firstPromptInBox`). When both conditions hold:
- `prevTop..top` is the actual composer (`history = lines[:prevTop]`, `draft = stripBoxChrome(lines[prevTop+1:top])`).
- `top..bottom` is treated as an auxiliary TUI drawer below the composer rather than draft text.

Existing 2-border composers (Claude Code, opencode, standard `agy` with 0 tasks) either already contain a prompt line inside `top..bottom` or have no adjacent prompt-headed box directly above `top`, so their behavior is completely unchanged.

## 2. `agy` Saved Conversation Titles & Placeholder `pane_title` Stripping

- **Annotation reader (`internal/transcript/agy.go` + `titles.go`)**:
  - `agy` writes per-conversation metadata to `$AGY_HOME/annotations/<conversation-id>.pbtxt` (typically `< 256` bytes, e.g. `title:"gtmux agy dev"`).
  - `AgySessionTitle(sessionID)` validates `safeAgySessionID(sessionID)`, reads at most `8 KiB`, and decodes the `title` string literal in pure Go (handling protobuf textformat escapes: `\n`, `\t`, `\"`, `\'`, `\\`, `\xHH` hex bytes, and `\OOO` 3-digit octal UTF-8 byte sequences), collapsing whitespace.
  - `SessionTitles("agy")` enumerates `$AGY_HOME/annotations/*.pbtxt` (bounded) for native rows and HQ workspace reads.
- **Radar integration (`internal/radar/agents.go`)**:
  - Strip generic `Antigravity` placeholder `pane_title` tokens (`"agy"`, `"antigravity"`, `"antigravity-cli"`, case-insensitive) in `classifyAgent` and subtree fallback so untitled panes leave `Task == ""` and display their tmux session name.
  - For bound `Antigravity` tmux panes (`boundResume(loc, agent)` where `rec.Agent == "agy"` and `cwd` matches when present), set `task` from `transcript.AgySessionTitle(rec.SessionID)` when non-empty.

## 3. Hook Lifecycle Ancestry, Subagent Isolation, Profile & Transcript Polish

- **Pre-detach canonical lifecycle (`internal/hook/hook.go`)**:
  - Before `shouldDetachSyncHook`, extract `hook_event_name`/`hookEventName` and `tool_name`/`toolCall.name` and resolve `originLifecycle := classifyPayload(agentKey, originEvent, originTool, raw).Lifecycle` (falling back to `originEvent` when empty).
  - Pass `originLifecycle` into `agentprofile.Observe` and `hookLaunchHost` (and `launchIdentityEvent`), so `PreInvocation` (`UserPromptSubmit` / `Resumed`) and `PostToolUse` (`Resumed`) record verified `execution_chain` and `launch_host` even when `SessionStart` occurred before hook installation.
- **Subagent hook guard (`internal/hook/hook.go`)**:
  - `agy` subagent invocations carry `rootConversationId` distinct from `conversationId`. Mirroring Codex's `agent_id` guard, return early (`0`) when `agentKey == "agy"` and `rootConversationId != ""` and `conversationId != ""` and `rootConversationId != conversationId`.
- **Profile `"auto"` guard & `AGY_HOME` scoping (`internal/agentprofile`)**:
  - In `HookFacts("agy")`, ignore `"modelName": "auto"` (case-insensitive) because `"auto"` is a router mode rather than a concrete model ID; concrete model IDs from `PLANNER_RESPONSE.model` (or explicit `modelName` selections) remain preserved.
  - Include non-empty `AGY_HOME` in `observationPath` for `agy` without altering default-home hashes when `AGY_HOME` is unset.
- **Transcript `<SYSTEM_MESSAGE>` & Activity normalization (`internal/transcript`)**:
  - Strip `<SYSTEM_MESSAGE>...</SYSTEM_MESSAGE>` blocks (and `agy`'s system-message preamble) in `CleanAgyUserContent` and `cleanAgyPromptText` so background task notifications never surface as fake user prompts.
  - Add `"search_web"` to `applyActivityFacts` (`Action: "web_search"`) and map `agy` tools (`grep_search`, `find_by_name`, `list_dir`, `view_file_outline`, `manage_task`, `manage_subagents`, `update_steps`, `call_mcp_tool`, `read_resource`, `list_resources`) cleanly in `agy.go` and `tool_input.go`.
