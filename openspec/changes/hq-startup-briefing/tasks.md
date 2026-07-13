# Tasks — HQ startup briefing

- [x] 1. `cmdHQ`: on fresh spawn, capture pane + raw agent command, then call `deliverHQBriefing` after the terminal tab opens (focused-live path returns before it)
- [x] 2. `deliverHQBriefing`: reuse `waitAgentReady` + `dispatch.Deliver` (via `dispatchIO`/`deliverOpts`); best-effort, non-fatal; no-op on empty pane or disabled
- [x] 3. `hqBriefingPrompt`: bilingual (`i18n.Tr`) two-part prompt — self-introduction + digest/usage/limits status report (needs-you first, token usage + subscription room)
- [x] 4. `hqBriefingEnabled`: opt-out via `GTMUX_HQ_BRIEF` (off/0/false/no), default on
- [x] 5. Tests: prompt carries both halves in en+zh; opt-out toggle parsing
- [x] 6. Spec delta: `supervisor-agent` new requirement + 4 scenarios
- [ ] 7. `make check` + `check-design` green; PR
- [ ] 8. Archive after merge (sync-specs → archive)
