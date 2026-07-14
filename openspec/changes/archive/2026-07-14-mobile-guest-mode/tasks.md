## 1. Server — confirm client-agnostic guest scope

- [x] 1.1 Add a test asserting a guest token used by a non-browser client is restricted identically (agents filtered, `/api/pane` 403 for non-viewable, `/api/send` gated) — pins the "scope is a token property" contract
- [x] 1.2 Confirm a guest token needs NO enroll to authenticate (it's already in the roster from `share new`); document that the app uses it directly as the bearer

## 2. App — pairing accepts a guest token

- [x] 2.1 Detect the credential KIND at ingest: a `#t=<token>` share link/QR → guest token (store as bearer, skip enroll); an enroll code / host+token → device via `POST /api/enroll` (unchanged owner path)
- [x] 2.2 Persist the pairing with its `scope` (owner/guest) in the Keychain alongside the bearer
- [x] 2.3 QR scan + pasted-URL entry both feed the same ingest path (per design Open Question)

## 3. App — scope model + guest-mode UI degradation

- [x] 3.1 On connect/reconnect/foreground, read `GET /api/share`; derive `isGuest` (`all:false`) + the `viewPanes`/`panes` sets into app state
- [x] 3.2 Central `isGuest` gate: hide owner-only surfaces — usage, digest/HQ console, device roster/management, share controls, Anywhere/tunnel/remote-access config
- [x] 3.3 Radar + pane + composer ride the already-guest-filtered endpoints; show the input affordance only on input-allowed panes (a non-viewable pane never renders)
- [x] 3.4 Suppress owner push-notification registration under guest scope (per design Open Question)

## 4. App — banner, fail-safe, revoke handling

- [x] 4.1 Persistent guest banner: "connected as a guest to `<host>` — N sessions"
- [x] 4.2 Fail safe on `403` from an owner-only endpoint → treat as "not permitted in this scope" (hide), never a fatal error
- [x] 4.3 On `401`/`403` from a CORE call (agents) — e.g. after `gtmux share revoke` — drop to the pairing screen instead of showing stale data

## 5. Tests + specs + docs

- [x] 5.1 App tests: token-kind detection (enroll code vs `#t=` guest token); scope detection (`all:true`→owner vs guest); owner-only screens absent under guest; input gated to allowlist
- [x] 5.2 `openspec validate --specs --strict` passes; archive the change after merge
- [x] 5.3 Docs: note the app is now a guest-capable client (scope is a token property, client-agnostic); update the scope model in remote-access docs / memory `web-shared-input`
