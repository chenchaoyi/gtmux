# Tasks — web-shared input (guest tokens + consent + per-pane allowlist)

## PR 1 — server core (the security gate)  ✅
- [x] `EnrolledDevice.Scope` (`""`⇒device / `guest`) + `MintGuest` + `TokenScope`; `auth()` resolves master/device/guest → request context
- [x] `ShareState{Enabled, Panes}` + `ShareManager` (share.go), persisted (`share.json`); wired in `serve.go`
- [x] `handleSend` gate: guest → allowed only if `Enabled && pane ∈ Panes`, else 403; master/device unchanged
- [x] `POST /api/share/config` + `POST /api/share/new` (master-only) + `GET /api/share` ({input, panes/all}); revoke reuses `/api/devices/revoke` (guests share the roster)
- [x] Tests: guest blocked off / not-allowlisted / allowed when both; owner unrestricted; capability per scope; master-only admin; `ShareManager.Allowed`

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
