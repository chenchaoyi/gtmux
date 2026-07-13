# Tasks — web-shared input (guest tokens + consent + per-pane allowlist)

## PR 1 — server core (the security gate)
- [ ] `EnrolledDevice.Scope` (`""`⇒full / `guest`); `auth()` resolves scope → request context
- [ ] `ShareState{Enabled, Panes}` persisted beside the roster; load/save in `serve.go`
- [ ] `handleSend` gate: guest → allowed only if `Enabled && pane ∈ Panes`, else 403; full unchanged
- [ ] Guest mint/revoke endpoints (master-authed); `GET /api/share` returns the caller's `{input, panes}`
- [ ] Tests: guest blocked when off / not-allowlisted / allowed when both; full unrestricted; `/api/share` per scope

## PR 2 — `gtmux share` CLI
- [ ] `gtmux share status|on|off|add <pane…>|remove <pane…>|new [--label]|revoke <id>` over the serve API (master token)
- [ ] `new` prints the share URL + QR (like pairing); tests

## PR 3 — web mirror input (`web/app.js`)
- [ ] Fetch `/api/share`; show a minimal input row (text + Enter + control keys) ONLY for allowed panes → `POST /api/send`
- [ ] Read-only unchanged when input is off; webui_test coverage

## PR 4 — menu-bar app UI (Swift)
- [ ] Consent toggle + per-pane picker + "new share link" / revoke, reading/writing the serve share API
- [ ] `swift build -c release` green

## Close-out
- [ ] Specs: `remote-access` (scope + send gate + `gtmux share` + guest links), `browser-mirror` (`/api/share` + web input)
- [ ] Each PR: make check + check-design green (+ swift build for PR4); archive `web-shared-input`
