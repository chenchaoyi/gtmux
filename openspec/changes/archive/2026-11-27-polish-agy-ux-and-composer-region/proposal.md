# Proposal: Polish Antigravity (`agy`) UX, Session Titles, Profile Metadata, and Multi-Box Composer Detection

## Why

Real-world daily usage of Antigravity CLI (`agy`) inside `gtmux` surfaced three concrete gaps:

1. **False-positive draft refusal during background tasks (`gtmux screenshot` / `gtmux send`)**: When `agy` runs background tasks (`1 task(s)`), it renders an auxiliary task-drawer box (`● [19:56:02] ... running`) between a second and third horizontal rule directly underneath the prompt composer box (`> `). `dispatch.SplitInputRegion` always took the bottom-most pair of horizontal borders, misreading the running task line as unsubmitted draft text (`ReachDraft`) and blocking `gtmux screenshot` and `gtmux send` even when the composer was empty.
2. **Placeholder `"agy"` task title and missing saved conversation titles**: `agy` sets `pane_title` to `"agy"`, causing radar surfaces to display `"agy"` as the primary bold session title instead of falling back to the user's tmux session name or showing the conversation's `/rename` / auto-generated title stored in `$AGY_HOME/annotations/<conversation-id>.pbtxt`.
3. **Incomplete profile/hook/transcript recognition**:
   - Pre-detach hook observation passed raw `agy` event names (`PreInvocation`, `PostToolUse`) before canonical classification, so `agentprofile.Observe` and `launchhost` skipped launch-process chain (`execution_chain`) and launch-host collection whenever `SessionStart` was not observed.
   - Hook payloads carrying `"modelName": "auto"` could overwrite the concrete `PLANNER_RESPONSE.model` recorded in `transcript.jsonl`, and subagent hooks carrying `rootConversationId != conversationId` were not isolated from the parent conversation.
   - System-injected `<SYSTEM_MESSAGE>` turns and `search_web` / `agy` workspace tools lacked complete filtering and Activity action normalization.

## What Changes

- **Composer region detection (`internal/dispatch/region.go`)**: Teach `SplitInputRegion` to recognize an auxiliary non-prompt drawer box rendered directly beneath a prompt-bearing composer box so `BoxEmpty`, `DraftOfColored`, and post-paste verification read the real composer box.
- **Conversation titles & placeholder stripping (`internal/transcript`, `internal/radar`)**: Parse `$AGY_HOME/annotations/<id>.pbtxt` (`AgySessionTitle` and `SessionTitles("agy")`), populate bound tmux and native `agy` row `Task` titles from saved annotations, and strip generic `"agy"` / `"antigravity"` / `"antigravity-cli"` placeholder `pane_title` values.
- **Hook ancestry, subagent isolation & profile accuracy (`internal/hook`, `internal/agentprofile`, `internal/transcript`, bilingual `docs/design/agent-profiles*.md`)**:
  - Classify `originLifecycle` before hook detachment so `PreInvocation` and `PostToolUse` capture verified `execution_chain` and `launch_host`.
  - Ignore `agy` subagent hooks (`rootConversationId != "" && rootConversationId != conversationId`).
  - Skip `"modelName": "auto"` as a concrete `model` fact in `HookFacts` and include `AGY_HOME` in profile storage scoping.
  - Strip standalone `<SYSTEM_MESSAGE>` injections in `CleanAgyUserContent` / `cleanAgyPromptText` and normalize `search_web` and `agy` discovery/task tools in `internal/transcript`.

## Surfaces

- **terminal**: `gtmux screenshot`, `gtmux send`, `gtmux agents`, `gtmux digest`, and HQ wakes accurately detect empty/non-empty `agy` composers and conversation titles.
- **menubar**: `Gtmux.app` rows show real `agy` `/rename` or auto-summary titles (or tmux session names when untitled) instead of `"agy"`, and screenshot-to-pane delivery succeeds while background tasks run.
- **phone**: Session list titles, Chat history (`<SYSTEM_MESSAGE>` excluded, `search_web` / tool actions normalized), Participant Profile (`Launch processes` chain + concrete model preserved under `auto`), and composer send guard work seamlessly.
- **iPad**: Split-view session list, Chat/Trace tool actions, and Participant Profile modal reflect accurate `agy` titles, launch chains, and models.
- **Web**: Browser session list, Chat/Trace activity rows, and `/api/participant-profile` dialog match mobile and CLI behavior.
