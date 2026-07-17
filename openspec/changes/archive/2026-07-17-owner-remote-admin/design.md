# Design: owner-remote-admin

## Context

`auth()` resolves every token to a scope (master / device / guest) in the request
context. Today: the three SHARE-management handlers call `masterOnly` (device +
guest both 403); `handleShare` (the caller's own capability) is any-scope;
`handleDevices` + `handleRevoke` have NO scope check (any valid token, including a
guest, passes). The mobile app connects with a device token (owner) or a guest
token; `AgentsContext.isGuest` already distinguishes them client-side.

## Goals / Non-Goals

**Goals (decision B):**
- An owner device manages SHARE exactly like the menu bar (consent, per-link
  scope, create, revoke-link).
- Owner can SEE the device roster (read-only).
- Close the guest can-list/revoke-any-device leak.

**Non-Goals:**
- Owner revoking paired devices, or toggling the remote door, from the phone
  (Mac/physical only).
- Any new persistence or token shape (reuse the roster + share state).

## Decisions

1. **`fullOnly` gate** = `callerScope != guest` (master OR device). Replaces
   `masterOnly` on `handleShareConfig`, `handleShareNew`, `handleShareSet`, and
   guards `handleDevices`. Rationale: an owner device is the commander's own
   surface — for SHARE management it is trusted like the master; the door/roster
   root operations are separately withheld (below).
2. **Scoped revoke.** `EnrollManager.RevokeBy(id, allowDevice bool)`: master calls
   with `allowDevice=true` (revoke anything); an owner device calls with
   `allowDevice=false` — the revoke succeeds ONLY if the target entry is a `guest`
   link, else it is refused (`403 forbidden: paired devices are managed on the
   Mac`). A guest caller is refused before reaching the manager. This both enables
   owner link-revocation AND closes the current unguarded path.
3. **No new endpoints.** The owner uses the SAME `/api/share/*` and `/api/devices`
   the menu bar uses; only the gate changes. The mobile client gains typed
   wrappers.
4. **Mobile screen gating.** The "Manage this Mac" entry appears only when
   `!isGuest` (the AgentsContext scope, already resolved authoritatively from
   `GET /api/share`). A guest never sees it; if a guest somehow calls the
   endpoints, the server refuses. The screen shows the door/roster as read-only
   facts (with a "manage on the Mac" note) — it does not offer the withheld
   actions, so the UI never dangles a button the server will 403.
5. **Reuse the menu bar's UX shape** for the share editor (per-link See/Type,
   Type ⊆ See enforced client-side and re-normalized server-side) so the two
   surfaces stay one mental model.

## Risks / Trade-offs

- **A lost/compromised owner phone can now re-scope sharing** (create links, widen
  allowlists, flip consent). Accepted (decision B): it CANNOT revoke devices or
  open the door, and the Mac owner can `gtmux pair revoke` the phone. This is the
  deliberate line between "manage sharing" (phone-allowed) and "re-key the machine"
  (Mac-only).
- **Guests lose device-list/revoke** they technically had (via the leak). This is
  a security fix, not a regression of any intended capability.
- **Owner device == master for share**: if finer separation is ever wanted, the
  gate is one function to split.

## Migration

- Pure authorization change; no data/format migration.
- Older mobile clients keep working (they simply don't show the new screen).
- A guest hitting the newly-guarded `/api/devices` now gets `403` instead of a
  list — intended.
