## ADDED Requirements

### Requirement: Guest-scope restriction is client-agnostic

The server's guest-scope filtering SHALL apply identically to ANY client holding a
`guest` token — a web browser and a native app alike. Scope is a property of the token,
not of the client surface: the same guest-filtered `GET /api/agents`, `403` on a
non-viewable `GET /api/pane`, `403` on `/api/usage` + `/api/digest`, suppressed SSE
`alert` events, and consent+allowlist-gated `POST /api/send` apply regardless of which
client presents the token. A native app connecting with a guest token is therefore a
first-class guest, restricted exactly as a guest browser.

#### Scenario: A native app with a guest token is restricted

- **WHEN** the gtmux mobile app connects with a `guest` token (not a `device` token)
- **THEN** it is restricted exactly as a guest browser: it sees only view-allowed panes
  in `/api/agents`, is refused (`403`) a non-viewable pane's `/api/pane`, and cannot
  `POST /api/send` outside the input allowlist

#### Scenario: A device token is unaffected

- **WHEN** the app connects with a `device` token (the owner's own paired phone)
- **THEN** it has full view + input, unaffected by any guest allowlist — identical to
  the master token
