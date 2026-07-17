# Proposal: owner-remote-admin

## Why

The pair/share model shipped (v0.27.0), but the management surface for it is
Mac-only: `gtmux share`/`gtmux pair` and the menu-bar Preferences run locally with
the master token. A PAIRED phone (an owner device — the commander's own surface,
full control) can drive agents but CANNOT manage sharing: creating/editing/revoking
share links and viewing the device roster all hit master-only or unguarded
endpoints. The commander wants the phone to have the same scoped-sharing management
the menu bar has (decision B, 2026-07-16): **owner may manage SHARE, but NOT the
device roster or the remote door** (revoking paired devices and toggling
serve/tunnel stay physical-Mac-only — a lost phone must not be able to re-key the
whole machine or cut off other devices).

While wiring this, a pre-existing gap surfaced: `GET /api/devices` and
`POST /api/devices/revoke` are behind `auth()` but carry NO scope check — a GUEST
token can currently list the roster and revoke ANY device (including the owner's).
This change closes that.

## What Changes

Server (authorization) + mobile (a management screen), decision B:

1. **A `full`-caller gate** (master OR owner device; guest refused) replaces
   master-only on the SHARE management endpoints, so an owner device may manage
   sharing: `POST /api/share/config` (consent + the global lists),
   `POST /api/share/new`, `POST /api/share/set`. `GET /api/devices` becomes
   `full`-only (fixing the guest leak) so the owner can SEE the roster.
2. **Scoped revoke**: `POST /api/devices/revoke` — a master may revoke anything;
   an owner device may revoke ONLY `guest` entries (share links), never a paired
   device (that stays master/physical-Mac only); a guest is refused entirely. This
   also fixes the current unguarded revoke.
3. **Mobile: a "Manage this Mac" screen** (owner-only; hidden for a guest
   connection). It surfaces, for the connected Mac: the SHARE controls (consent
   toggle · per-link See/Type editor · a create-share sheet · revoke a link) —
   mirroring the menu bar — plus a READ-ONLY paired-device list (no revoke, no
   door: those are Mac-only by decision B, with a one-line note saying so).
   New client methods: `devices()`, `shareConfig()/setShareConfig()`,
   `shareNew()`, `shareSet()`, `revokeShare()`.

Out of scope (decision B): revoking paired devices from the phone; toggling the
remote-access door (serve/tunnel) from the phone; `gtmux pair` minting from the
phone.

## Capabilities

### Modified Capabilities
- `remote-access`: an explicit authorization matrix — the SHARE management
  endpoints accept `full` callers (master + owner device), `GET /api/devices` is
  `full`-only, and `/api/devices/revoke` is scoped (owner ⇒ guest entries only).
- `mobile-app`: an owner-only remote-management screen (scoped-sharing controls +
  a read-only device roster) for the connected Mac.

## Impact

- Server: `internal/server/server.go` (a `fullOnly` gate + scoped revoke wiring),
  `enroll.go` (revoke-by-id honoring the caller's scope), `share.go` (swap
  `masterOnly` → `fullOnly` on the three share-management handlers).
- Mobile: `mobileapp/src/api/client.ts` (management methods), a new
  `ManageMacScreen` + navigation entry gated on `!isGuest`, i18n.
- Docs: `api/contract.md` (the authorization notes on those endpoints),
  `docs/cli.md` unaffected.
- Specs: remote-access, mobile-app.
- Security: closes the guest can-list/revoke-devices leak (a hardening, shipped in
  the same change).
