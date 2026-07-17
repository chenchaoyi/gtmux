# Tasks: push-token-device-binding

## 1. Server — bind tokens to devices + revoke drops them

- [x] 1.1 `DeviceToken.DeviceID string` (`json:"deviceId,omitempty"`) in
      `internal/server/push.go`. Loads back-compat (empty on legacy files).
- [x] 1.2 `handleRegister` stamps `d.DeviceID` from
      `s.deps.Enroll.DeviceByToken(bearerToken(r))` (nil-safe; empty if not found).
- [x] 1.3 `PushManager.UnregisterByDevice(id string)` — drop every token with that
      `DeviceID`, persist once if changed; empty id is a no-op.
- [x] 1.4 `handleRevoke` (enroll.go) calls `s.deps.Push.UnregisterByDevice(body.ID)`
      after a successful `RevokeBy` (nil-safe).
- [x] 1.5 Tests: register binds the caller's id; revoke drops the bound token and stops
      pushing to it; an empty-id revoke drops no unlinked tokens.

## 2. Server — inspect + clear (master-only)

- [x] 2.1 `GET /api/push/tokens` (`handleTokens`) — master-only; returns
      `{tokens:[{deviceId, tokenPrefix, platform, env, kinds}]}` REDACTED.
- [x] 2.2 `POST /api/push/forget` (`handleForget`) — master-only; `{deviceId|orphans|all}`
      → drop + persist + `{forgotten:n}`. Non-master → 403.
- [x] 2.3 Routes in `server.go`; `PushManager.ForgetBy(sel)` / reuse
      `UnregisterByDevice` + an orphans/all path.
- [x] 2.4 Tests: redaction (never the full token); orphans drops only empty-id tokens;
      deviceId/all selectors; non-master 403.

## 3. CLI — gtmux devices --push / --forget-push

- [x] 3.1 `internal/app/devices.go`: `--push` lists the roster annotated with each
      device's push binding (env·kinds or —) + "N unlinked push token(s)".
- [x] 3.2 `--forget-push <device-id|orphans|all>` → `POST /api/push/forget`.
- [x] 3.3 Test the CLI arg parsing / request shape where practical (pure mapper).

## 4. Consistency + verification

- [x] 4.1 api/contract.md: `DeviceToken.deviceId`, `GET /api/push/tokens`,
      `POST /api/push/forget`, and the revoke-drops-token note.
- [x] 4.2 docs/cli.md: `gtmux devices --push` / `--forget-push`.
- [ ] 4.3 Fold spec deltas (push-notifications, remote-access); `openspec --strict`
      green; archive change.
- [x] 4.4 `make check` + `CGO_ENABLED=0 go build ./cmd/gtmux` + `check-design.sh` green.
- [ ] 4.5 Dogfood: register from the phone, `gtmux devices --push` shows it bound;
      revoke → it disappears + no more push; `--forget-push orphans` clears a legacy one.
