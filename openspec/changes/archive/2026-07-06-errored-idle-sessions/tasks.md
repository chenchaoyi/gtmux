## 1. Detection (internal/transcript)

- [x] 1.1 Add `LastMessageError(agent, sessionID) (errored bool, text string)` in
      `internal/transcript/`: read the same 64KB tail as `LastMessageTime`, find the
      last non-meta/non-sidechain line, return `(true, summary)` iff it is a Claude
      `assistant` line with `isApiErrorMessage: true`; extract a short `error_text`
      (the `API Error: …` head, truncated). Non-Claude/unavailable → `(false, "")`.
- [x] 1.2 Parse `isApiErrorMessage` + the error content in `internal/transcript/claude.go`
      (extend `claudeLine`/`claudeMessage`); ignore mid-turn error lines that are not last.
- [x] 1.3 Unit tests: last-line-error → true+text; recovered mid-turn error → false;
      normal end_turn → false; non-Claude/empty → false; text truncation.

## 2. Contract (internal/app/agents.go)

- [x] 2.1 Add `Error bool json:"error,omitempty"` + `ErrorText string json:"error_text,omitempty"`
      to `agentJSON`.
- [x] 2.2 For idle rows with a readable Claude transcript (reuse the existing
      resume/session-id path used for the idle clock), call `LastMessageError` and set
      the fields. Leave absent for non-idle / non-Claude / unreadable.
- [x] 2.3 Contract/agents tests: an idle row whose transcript ends in an error gets
      `error:true`+`error_text`; a normal idle row has neither; JSON stays backward compatible.

## 3. CLI surface (internal/app)

- [x] 3.1 In the `agents` text output, render an errored idle row as `⚠ errored`
      (amber) with the `error_text` summary where a normal idle prints `✳ idle`.
- [x] 3.2 en+zh strings via `internal/i18n` for the "errored" label.

## 4. Menu-bar surface (macapp)

- [x] 4.1 Add `error`/`errorText` to `AgentStore.swift` `Agent` (tolerate missing).
- [x] 4.2 In the row/`AgentState`, swap the green ✓ for an amber ⚠ + summary when
      `error` is set; keep it in the IDLE section; do NOT use red. Add the amber
      modifier hex next to the state palette.

## 5. Mobile surface (mobileapp)

- [x] 5.1 Add `error`/`error_text` to `src/api/types.ts` `Agent` + `toAgent`.
- [x] 5.2 Render the errored idle row with an amber ⚠ + summary (theme modifier
      color, not a state color); keep it in the idle section; not red.
- [x] 5.3 Unit tests for `toAgent` decoding the new fields.

## 6. Gate + docs

- [x] 6.1 `make check` (Go) and `cd mobileapp && npm run check` green; `cd macapp &&
      swift build -c release`.
- [x] 6.2 Update `docs/cli.md` (agents row legend) + note the errored modifier; keep
      `agents --json` contract doc in sync.
- [x] 6.3 `npx @fission-ai/openspec validate --specs --strict` passes; sync/archive
      the change.
