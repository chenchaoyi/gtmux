## Context

gtmux maps the Claude Code `Stop` hook straight to `idle` (`internal/hook/hook.go`
`decide`), and `internal/app/agents.go` builds idle rows without inspecting the
transcript beyond `transcript.LastMessageTime` (used only for the "finished N ago"
clock). Claude Code records an API/tool failure as a transcript line
`{"type":"assistant", "isApiErrorMessage":true, "error":…, "message":{content:"API
Error: …"}}`. Empirically (543 local sessions) 7 ended with that as the **last**
line; the mid-turn retry errors that recovered have a normal last line. So "ended
in error" is reliably the **last-line** test, not "contains an error".

## Goals / Non-Goals

**Goals:**
- Detect an idle session that ended on an API error and expose it as `error` +
  `error_text` on the `agents --json` contract (backward compatible).
- Render it on all three surfaces as an amber ⚠ "errored" modifier (+ summary),
  replacing the green ✓, without leaving the idle state/section.

**Non-Goals:**
- No new `status` value; no re-sorting (errored stays in idle order).
- Not red — red is reserved for `waiting`. Amber ⚠ is a modifier, like `latest`.
- No detection for non-Claude agents in v1 (Codex/Gemini transcripts don't carry
  `isApiErrorMessage`); the flag is simply absent for them.
- No notification/push for errored idle in v1 (radar-only signal). Can follow later.

## Decisions

- **Detection lives in `internal/transcript`**: add `LastMessageError(agent,
  sessionID) (bool, string)` alongside `LastMessageTime`, reading the same 64KB
  tail. It scans forward to the last non-sidechain/non-meta line and returns
  `(true, summary)` iff that line has `isApiErrorMessage: true`, extracting a short
  `error_text` (the `API Error: …` head, truncated). Non-Claude agents → `(false,"")`.
- **Wiring in `agents.go`**: for rows resolved to `idle` with a readable Claude
  transcript, call the check and set `agentJSON.Error` / `agentJSON.ErrorText`.
  Reuse the same `resume.Load`/session-id path already used for the idle clock, so
  no extra tmux work.
- **Contract**: add `error bool` (omitempty) and `error_text string` (omitempty) to
  `agentJSON`. Mirror in `AgentStore.swift` `Agent` and `mobileapp/src/api/types.ts`
  `Agent` (both already tolerate missing fields).
- **Surfaces**: CLI `agents` prints `⚠ errored` (amber) with the summary where a
  successful idle prints `✳ idle`; menu-bar `AgentState`/row swaps the green ✓ for
  an amber ⚠ + summary; mobile radar row does the same. Amber hex is a NEW modifier
  color (not a state color) — pick one shared value (e.g. `#F59E0B`) and document it
  next to the state palette; it is used ONLY for the ⚠ error marker.

## Risks / Trade-offs

- **False "errored"** if Claude leaves a stale error line last but the user has
  since acted: acceptable — the next `UserPromptSubmit`/`Stop` re-reads the tail and
  clears it, same lifecycle as other transcript-derived signals. `SessionStart`/
  resume also moves the last line past the error.
- **Tail cost**: one extra 64KB tail read per idle Claude row per radar build; the
  idle clock already reads the same file, so fold both into one read if profiling
  shows it (optimization, not required for correctness).
- **Amber vs the state-color invariant**: mitigated by scoping amber to the ⚠
  MODIFIER only (never a row/state fill), documented alongside the palette so future
  work doesn't mistake it for a status color.
