## Why

The gtmux mobile app pairs only via the enroll/QR flow → a `device`-scope token =
FULL owner access (unrestricted view + input), the same as the Mac. There is no way
to hand someone a **scoped** session on the native app: the view/input allowlist
shipped in web-shared-view-scope governs only `guest`-scope **web** share links. This
makes the app a **guest-capable client** so a collaborator can use the native app but
see and type only what the host allowed — reusing the existing sharing primitive and
permission model, not a new one.

## What Changes

- The app accepts the **same guest link** `gtmux share new` already mints
  (`<base>/#t=<token>` + its QR). On receiving a credential it detects whether it is
  an **enroll code** (→ device, full, via `POST /api/enroll`) or a **guest token**
  (→ guest, restricted, used directly as the bearer, no enroll). One artifact, both
  web and app.
- On connect the app reads `GET /api/share` to learn its scope: `all:true` ⇒ owner
  (full, today's behavior); otherwise it is a **guest** scoped to the returned
  `view_panes` (viewable) and `panes` (typable).
- **Guest mode UI degradation:** the app shows only viewable sessions (its radar
  already renders from the guest-filtered `/api/agents`), input only on typable
  panes, and **hides owner-only surfaces** — usage, the digest/HQ command console,
  device roster/management, the share controls, and Anywhere/tunnel/remote-access
  config. It **fails safe**: never calls owner-only endpoints (they `403`) and adapts
  rather than erroring.
- A persistent **guest banner** ("connected as a guest to `<host>` — N sessions")
  makes the restricted scope unambiguous.
- **Security:** a guest token in the app is the same secret as a guest web link —
  stored like the existing bearer, revocable by the host via `gtmux share revoke`,
  and governed by the same per-pane allowlists (input ⊆ view).

### Non-goals / future

- **(North star, NOT this change) A native remote TERMINAL client** (`gtmux connect
  <remote>`) that drives the Mac's sessions from another machine's terminal, in owner
  OR guest mode over the same HTTP/SSE contract — a separate, larger capability (a new
  TUI client + transport; overlaps the deferred SSH/mesh direction). This change is
  the stepping stone: it proves "any surface × either scope" and makes the app the
  first non-browser client that can be either owner or guest.
- No change to how the **owner** pairs their own phone (still device/full via enroll).
- No per-guest-at-pairing-time allowlist UI — the host scopes via the existing `gtmux
  share` / menu-bar allowlist; a guest phone is governed by the same allowlists as a
  guest browser.

## Capabilities

### New Capabilities
<!-- none — extends the existing mobile-app + remote-access capabilities -->

### Modified Capabilities
- `mobile-app`: pairing can accept a guest token (scanned share link/QR), and the app
  runs in a scope-aware GUEST mode — restricted radar/input, owner-only surfaces
  hidden, a guest banner, fail-safe on `403`.
- `remote-access`: state explicitly that guest-scope filtering is **client-agnostic**
  — any client holding a guest token (browser or native app) is restricted identically
  by the server; the app is a first-class guest client.

## Impact

- **Mobile app (RN/TS):** the pairing flow (detect enroll-code vs guest-token; store a
  guest bearer without enroll), a scope model read from `GET /api/share`
  (`all` ⇒ owner), conditional rendering that hides owner-only screens under guest, the
  guest banner, and 403-tolerant data calls. Reuses the existing bearer storage +
  `/api/agents`/`/api/pane`/`/api/send` clients.
- **Server (Go):** ~none — guest-scope filtering already applies to any client. At most
  a clarifying test that a guest token used by a non-browser client is restricted.
- **Docs/specs:** mobile-app + remote-access deltas; note in the deploy/scope docs.
- **Contracts:** additive — reuses `GET /api/share` (`all`/`view_panes`/`panes`) and
  the `#t=<token>` link; no new endpoints.
