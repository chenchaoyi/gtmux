# Tasks — web-shared input (guest tokens + consent + per-pane allowlist)

## PR 1 — server core (the security gate)  ✅
- [x] `EnrolledDevice.Scope` (`""`⇒device / `guest`) + `MintGuest` + `TokenScope`; `auth()` resolves master/device/guest → request context
- [x] `ShareState{Enabled, Panes}` + `ShareManager` (share.go), persisted (`share.json`); wired in `serve.go`
- [x] `handleSend` gate: guest → allowed only if `Enabled && pane ∈ Panes`, else 403; master/device unchanged
- [x] `POST /api/share/config` + `POST /api/share/new` (master-only) + `GET /api/share` ({input, panes/all}); revoke reuses `/api/devices/revoke` (guests share the roster)
- [x] Tests: guest blocked off / not-allowlisted / allowed when both; owner unrestricted; capability per scope; master-only admin; `ShareManager.Allowed`

## PR 2 — `gtmux share` CLI  ✅
- [x] `gtmux share status|on|off|add|remove|new [--label]|revoke` over the serve master API; `handleShareConfig` gains GET (master state)
- [x] `new` prints the guest share link (`<tunnel|serve>/#t=<token>`) + QR; guests filtered from the roster by scope; GET-config master-only test

## PR 3 — web mirror input (`web/app.js`)  ✅
- [x] boot reads a guest `#t=<token>`; `fetchShare()` learns `{input, panes, all}`; `paneCanInput` + `updateInputBar`
- [x] `#pane-input` row (text+Enter + allowlisted control keys) shown ONLY for allowed panes → `POST /api/send` (403 → "not shared"); read-only otherwise

## PR 4 — menu-bar app UI (Swift)  — DEFERRED (user, v1 = CLI)
- Consent toggle + per-pane picker + "new share link" / revoke in the menu-bar app.
  Deferred to a future change; v1 ships the CLI as the control surface. The
  `remote-access` spec delta notes the menu-bar surface as a planned follow-up.

## Close-out
- [x] Specs: `remote-access` (scope + send gate + `gtmux share` + guest links), `browser-mirror` (`/api/share` + web input)
- [x] Each shipped PR: make check + check-design green
- [ ] Archive `web-shared-input` (PR1-3 shipped; menu-bar UI its own future change)
