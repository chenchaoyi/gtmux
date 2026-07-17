# Proposal: push-token-device-binding

## Why

A serve stores each registered APNs push token STANDALONE in
`~/.config/gtmux/push-tokens.json` (`DeviceToken{token, platform, env, kinds}`) with
NO link to the enrolled device that registered it. Two consequences bit a real user:

1. **Revoking a device does NOT stop its push.** The roster (enroll) and the push-token
   store are separate; `POST /api/devices/revoke` removes the roster entry but leaves
   the token, so a removed/estranged device keeps receiving notifications. A colleague's
   Mac kept pushing to a phone that had unpaired from it — the only stop was
   hand-editing `push-tokens.json` on that Mac and restarting serve.
2. **There is no way to inspect or clear registered push tokens** from the CLI. Tokens
   are opaque, so even the hand-edit can't tell which token belongs to whom.

`gtmux update`'s phone-side unregister (#455) only helps when the phone STILL has the
server (it can call `/api/push/unregister`); it does nothing for tokens already
stranded on someone else's Mac, or for old-app removals.

## What Changes

1. **Bind tokens to devices.** `DeviceToken` gains a `DeviceID` (the roster id of the
   enrolled device that registered it). `POST /api/push/register` stamps it from the
   caller's roster entry (`Enroll.DeviceByToken(bearerToken(r))`).
2. **Revoke drops the token.** `POST /api/devices/revoke` calls a new
   `PushManager.UnregisterByDevice(id)` after removing the roster entry, so revoking a
   device (or a guest link) immediately stops its notifications — no file editing.
3. **Inspect + clear from the CLI.** A master-only `GET /api/push/tokens` (redacted:
   token prefix only) and `POST /api/push/forget {deviceId|orphans|all}` back a
   `gtmux devices --push` view (the roster annotated with each device's push binding +
   env/kinds, plus a count of UNBOUND legacy tokens) and a
   `gtmux devices --forget-push <device-id|orphans|all>` clear path. `orphans` targets
   exactly the pre-binding legacy tokens (the estranged-colleague cleanup) without
   touching bound ones.

Legacy tokens (registered before this change, empty `DeviceID`) are additive/back-compat:
they keep working, are shown as "unlinked", are NOT dropped by revoke (no id to match),
and are cleared with `orphans`/`all`.

## Capabilities

### Modified Capabilities
- `push-notifications`: push tokens are bound to the enrolled device that registered
  them; revoking that device drops the token; the token store is inspectable/clearable.
- `remote-access`: `POST /api/devices/revoke` now also unregisters the revoked device's
  push token(s); new master-only `GET /api/push/tokens` + `POST /api/push/forget`.

## Impact

- Server: `internal/server/push.go` (`DeviceToken.DeviceID`, `handleRegister` stamps it,
  `UnregisterByDevice`, `handleTokens`, `handleForget`), `internal/server/enroll.go`
  (revoke wires `Push.UnregisterByDevice`), `internal/server/server.go` (routes).
- CLI: `internal/app/devices.go` (`--push` list, `--forget-push` clear).
- Docs: `api/contract.md` (DeviceToken.deviceId, the two new endpoints, revoke note),
  `docs/cli.md` (`gtmux devices --push`/`--forget-push`).
- Specs: push-notifications, remote-access.
- Migration: none required; the new field is additive (`omitempty`), legacy tokens
  keep authenticating and are cleared via `orphans`/`all`.
