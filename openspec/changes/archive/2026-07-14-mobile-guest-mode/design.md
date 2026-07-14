## Context

`gtmux serve` resolves every request to a token scope — `master`, `device` (an
owner-paired phone, full), or `guest` (a share link, restricted by the view/input
allowlists shipped in web-shared-view-scope). The web page already works as either an
owner or a guest client (it reads `#t=<token>` and `GET /api/share`). The mobile app,
however, only ever pairs as a `device` (full owner). Because the server enforces guest
scope for ANY client, making the app a guest client is almost entirely app-side work:
detect the token kind, read the scope, and degrade the UI.

## Goals / Non-Goals

**Goals:**
- Let a collaborator use the native app scoped to the host's view/input allowlists.
- Reuse the existing sharing primitive (`gtmux share` link/QR) and permission model —
  no new server scope, no new token type.
- The app fails safe under guest scope (never leaks owner-only data or crashes on 403).

**Non-Goals:**
- A native remote TERMINAL client (`gtmux connect`) — the north star this unlocks, but a
  separate capability (new TUI client + transport). Not built here.
- Changing owner pairing (still `device`/full via enroll).
- A per-guest allowlist chosen at pairing time — the host scopes via `gtmux share`.

## Decisions

- **One sharing artifact for web + app (decision A).** The app ingests the same
  `<base>/#t=<token>` link/QR `gtmux share new` mints. No guest-specific pairing
  endpoint. Alternative (a dedicated guest-pairing code) rejected: it doubles the
  sharing surface for no gain; the `#t=` link already carries a scoped, revocable token.
- **Detect token KIND at ingest.** A pairing QR/entry can be (i) an enroll code → redeem
  via `POST /api/enroll` → device token; or (ii) a `#t=<token>` guest link → use the
  token directly as the bearer, skip enroll. The app branches on the input shape (a
  `#t=` URL vs an enroll code/host+token), so the user never picks "owner vs guest" — the
  artifact decides.
- **Scope from `GET /api/share`, not a new field.** `all:true` ⇒ owner; else guest with
  `view_panes`/`panes`. This endpoint already returns exactly this, so no contract
  change. The app stores `scope` alongside the pairing and re-reads it on reconnect.
- **UI degradation is a render-time gate, not a separate build.** A single
  `isGuest` (derived from scope) hides owner-only screens/tabs and suppresses their data
  calls. The radar/pane/send paths are unchanged — they already ride the guest-filtered
  endpoints, so a guest simply sees less.
- **Fail safe on 403.** Any owner-only call that slips through returns `403`; the app
  treats `403` on an owner endpoint as "not permitted in this scope" (hide the feature),
  never as a fatal error. A `401`/`403` on a CORE call (agents) after a revoke returns
  the app to the pairing screen.

## Risks / Trade-offs

- [A guest token is a bearer password in the app] → Store it like the existing device
  bearer (Keychain); the host revokes via `gtmux share revoke`; the banner keeps the
  scope visible. Same exposure as a guest web link, no worse.
- [Owner-only surface leaks if a screen forgets the `isGuest` gate] → Centralize the gate
  (one `isGuest` selector) and add a test asserting owner-only screens are absent under
  guest scope; prefer "hide by default, show for owner" for the sensitive screens.
- [Revoke isn't noticed until the next call] → On a `401`/`403` from a core call, drop to
  pairing; the SSE reconnect + foreground refetch surface it promptly.
- [Scope drift: host widens/narrows the allowlist mid-session] → The app re-reads
  `GET /api/share` on reconnect/foreground so the visible scope tracks the host.

## Migration Plan

Purely additive and app-side. Existing owner pairings keep working (they read
`all:true`). No server migration; no share.json change. Ships in the next mobile device
build (the app is not tag-released).

## Open Questions

- Where does the app read the guest link — QR scan only, or also a pasted `#t=` URL /
  universal link? Proposed: both (scan + paste), same code path. Confirm during apply.
- Should a guest app suppress push-notification registration entirely, or register but
  only for its scoped sessions? Proposed: suppress for the MVP (owner alerts are
  owner-only); revisit if guests want their own notifications.
