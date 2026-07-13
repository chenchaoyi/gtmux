# Design — web-shared input

## Token scope (the security hinge)

`auth()` already knows whether a token is the master or an enrolled device; it just
discards that. Change it to resolve a SCOPE and put it in the request context:

- `full` — the master token, and any paired device (the owner's own trusted surfaces).
- `guest` — a share-link token (a new roster entry with `scope:"guest"`).

`EnrolledDevice` gains `Scope string` (empty ⇒ `full`, back-compat for existing
devices). Guest tokens live in the SAME roster (revocable, persisted the same way),
minted by a separate path so a pairing QR never yields a guest and vice-versa.

## Consent + allowlist (host-controlled, persisted)

A small `ShareState{ Enabled bool; Panes []string }` persisted beside the device
roster (serve state dir). `Enabled` defaults false; `Panes` is the pane-id allowlist.
The owner edits it via `gtmux share` (and it is exposed read-only to the web via
`/api/share`). Pane ids are stable enough for a session; a pane that disappears is
simply ignored (send fails naturally).

## The `/api/send` gate

`handleSend` reads the scope from context:
- `full` → unchanged (unrestricted).
- `guest` → allowed ONLY if `ShareState.Enabled && pane ∈ ShareState.Panes`, else
  `403 forbidden` (a distinct, non-leaky message). This is the authoritative gate.

## `GET /api/share` (capability for the web UI)

Returns, for the CALLER: `{ input: bool, panes: []string }` — for a guest, `input` is
`ShareState.Enabled` and `panes` the allowlist; for a full token, `input:true` and
`panes` = all (they can type anywhere). The web page renders an input affordance ONLY
for panes this returns, so a guest never even sees an input box for a disallowed pane —
but the server gate is what actually enforces it.

## `gtmux share` command

- `gtmux share status` — show consent + allowlist + guest links.
- `gtmux share on|off` — toggle consent.
- `gtmux share add <pane…>` / `remove <pane…>` — edit the allowlist.
- `gtmux share new [--label X]` — mint a guest token; print the share URL (the serve/
  tunnel base + the guest token) + a QR, like pairing.
- `gtmux share revoke <label|token-prefix>` — kill one guest link.

It talks to the running `serve` over its local API (the master token), the same way
`gtmux devices` manages the roster.

## Web mirror input (`web/app.js`)

On load, fetch `/api/share`. For an allowed pane, show a minimal input row (text +
Enter + a few control keys) that POSTs `/api/send` — reusing the mobile app's proven
request shape. No input UI for disallowed panes. The read-only path is unchanged when
input is off.

## Forks — RESOLVED (user, 2026-07-13)

- **F1 → Guest share tokens.** A new `guest` scope; the owner mints share links that are
  input-restricted to the allowlist + consent; master + devices stay full and are each
  individually revocable. (Not the global toggle — a guest must be grantable VIEW without
  INPUT, revocable on its own, and a shared full token must never bypass the allowlist.)
- **F2 → CLI `gtmux share` AND the menu-bar app UI.** Both: the CLI for scripting +
  headless, and a menu-bar consent toggle + pane-picker + share-link surface.

Decided by the request (not forks): consent required, defaults OFF; per-pane allowlist.
