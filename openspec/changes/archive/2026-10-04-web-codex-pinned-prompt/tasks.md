## Implementation

- [x] Shared case fixture (`mobileapp/src/ui/codexPinnedCases.json`) run by the phone's tests.
- [x] JavaScript matcher and width table in `internal/server/web/app.js`, run against the same
  fixture by `app.test.cjs`; a test that the JS and TS width tables are identical.
- [x] The bar in the single-pane view (`index.html`, `style.css`): two lines, click to open,
  Copy; the matched row is left out of the terminal write.
- [x] Terminal-mode transcript fetch while the row is unexplained, at most every 4 s.
- [x] browser-mirror spec, WEB.md and WEB.zh.md.
- [x] Sync specs and archive this change.

## Acceptance

Not claimed: a check in a real browser against a live Codex pane. Recorded here, not as a task.
