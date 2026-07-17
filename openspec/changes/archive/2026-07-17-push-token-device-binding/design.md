# Design: push-token-device-binding

## Context

- Push tokens live in a `PushManager` (in-memory `[]DeviceToken`, persisted to
  `~/.config/gtmux/push-tokens.json` via a `save` hook). `DeviceToken` today is
  `{token, platform, env, kinds}` — no owner.
- The roster is a separate `EnrollManager` (`EnrolledDevice{id, name, token, scope, …}`,
  keyed by token). `DeviceByToken(tok)` returns the entry for a bearer token;
  `bearerToken(r)` extracts it from the request. `auth()` stashes the roster entry in
  ctx ONLY for guest scope, so `handleRegister` (called by a device/owner) can't read
  it from ctx — it must look up via `DeviceByToken(bearerToken(r))`.
- `handleRevoke` → `EnrollManager.RevokeBy(id, allowDevice)` removes the roster entry
  but never touches the push store.
- Push registration is an OWNER capability (the mobile app gates the notification
  settings on `!isGuest`), so the registering caller is a paired DEVICE with a roster
  id. The MASTER token (the Mac itself) is never a push target.

## Goals / Non-Goals

**Goals**
- Revoking a device drops its push token(s) automatically (no hand-editing).
- Inspect + clear the token store from the CLI on the Mac (master token).
- Fully back-compat: legacy unbound tokens keep working and are clearable as orphans.

**Non-Goals**
- Pruning tokens on APNs 410/BadDeviceToken (the fire-and-forget relay gives the serve
  no async delivery feedback; a separate change if wanted).
- Any phone-side change (the mobile client already sends the bearer token on register;
  the server derives the binding — no new client field).
- Multi-token-per-device semantics beyond "drop all tokens for this device id".

## Decisions

1. **`DeviceToken.DeviceID string` (`json:"deviceId,omitempty"`).** Stamped
   server-side at register from `Enroll.DeviceByToken(bearerToken(r))`; empty when the
   caller isn't a roster device (shouldn't happen for real push, and stays empty for
   legacy on-disk tokens). Additive → old files load unchanged.
2. **`PushManager.UnregisterByDevice(id string)`** drops EVERY token whose `DeviceID`
   matches (a device could register more than once across reinstalls) and persists once
   if anything changed — mirrors `Unregister(token)`. An empty `id` is a no-op (so a
   legacy revoke can't accidentally nuke all unbound tokens).
3. **Revoke wires it.** `handleRevoke`, after a successful `RevokeBy`, calls
   `s.deps.Push.UnregisterByDevice(body.ID)` (nil-safe). Guests (share links) are
   revoked the same way — if a guest ever held a token, it's dropped too.
4. **Inspect/clear endpoints (master-only).**
   - `GET /api/push/tokens` → `{tokens:[{deviceId, tokenPrefix, platform, env, kinds}]}`
     — REDACTED (first ~8 chars of the token only; never the full secret).
   - `POST /api/push/forget {deviceId?|orphans?|all?}` → drops matching tokens, persists,
     returns `{forgotten:<n>}`. `orphans` = tokens with empty `DeviceID`; `all` = every
     token; `deviceId` = that device's tokens. Master-only (`callerScope==master`) — a
     device/guest is 403 (they manage their own token via register/unregister only).
5. **CLI surface on `gtmux devices`.** `--push` renders the roster with a per-device
   push column (✓ env·kinds, or "—") and, below it, "N unlinked push token(s)" when
   orphans exist. `--forget-push <device-id|orphans|all>` posts to `/api/push/forget`.
   Chosen over a new `gtmux push` command to keep the roster + its push state in one
   place (the CLAUDE.md command list already carries `devices`).
6. **The estranged-colleague fix, end to end.** New world: the phone unpairs → owner
   revokes that device on the Mac → its token is dropped (decision 3). Legacy world (the
   token predates binding): `gtmux devices --push` shows it as unlinked; `gtmux devices
   --forget-push orphans` clears it (and the serve re-persists) — no file edit, no
   blunt whole-file wipe.

## Risks / Trade-offs

- **A device that re-registers after revoke** re-adds its token (expected — it's paired
  again). Not this change's concern.
- **`all` is blunt** (drops every device's token; they re-register on next app open).
  It's an explicit, documented escape hatch, not the default.
- **Redaction**: `GET /api/push/tokens` must never return the full token — only a prefix
  for human matching. Enforced in the handler + pinned by a test.

## Migration

- Pure additive field; no format migration. Old `push-tokens.json` loads with
  `DeviceID==""` and those tokens are treated as orphans.
- Older mobile clients need no change — the server derives `DeviceID` from their bearer
  token at register.
