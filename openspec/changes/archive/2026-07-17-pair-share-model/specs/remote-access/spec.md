# remote-access — delta

## MODIFIED Requirements

### Requirement: Guest tokens with consented, per-pane shared input

The system SHALL let the host share the web page such that a collaborator SEES only the
panes the host chose to show and can type into the terminal ONLY with the host's explicit
consent and ONLY into panes the host selects, without ever granting that collaborator the
host's own full control. Every credential SHALL carry a SCOPE: `full` (the master token
and paired devices — the owner's own surfaces, unrestricted view and input) or `guest` (a
share link). The product vocabulary is two-track: PAIR (配对) names the owner's own
surfaces, SHARE (分享) names collaborator links.

Each guest link SHALL carry its OWN two per-pane allowlists — a **view** allowlist
(which panes that guest may SEE) and an **input** allowlist (which panes that guest may
TYPE into) — so different collaborators can be granted different sessions and different
verbs. The invariant **input ⊆ view** SHALL hold per link: allowing input on a pane
SHALL imply view on that pane, and removing view SHALL remove its input. Shared-input
CONSENT SHALL remain one host-level master switch (default OFF) gating ALL guest typing.
`POST /api/send` SHALL enforce the scope: a `full` caller is unrestricted; a `guest`
caller SHALL be allowed only when consent is ON and the target pane is on THAT LINK's
input allowlist, else `403`. All gates are server-side and authoritative.

The default SHALL be secure: consent off, and a link minted with no explicit scope in a
fresh install sees NO panes and may type into NONE.

Compatibility semantics for the legacy GLOBAL allowlists SHALL be: (1) MIGRATION — on
serve start, any guest link lacking per-link scope fields receives a one-time copy of
the legacy global lists; (2) TEMPLATE — a link minted without explicit scope flags
copies the current global lists; (3) BROADCAST — the legacy global mutations
(`gtmux share add/remove`, `share view add/remove`, the share config POST) update the
template AND fan out to every existing guest link, so pre-existing UIs keep their exact
observed behavior.

The host SHALL control this via `gtmux share`: `status` (per-link scope summaries);
`on/off` (consent); `new --label <name> [--view <panes>] [--type <panes>]
[--expires <dur|never>]` (mint with scope in one step); `set <id> [--view …] [--type …]
[--expires …]` (edit one link; an omitted flag leaves that facet untouched); `revoke
<id>`; plus the legacy global forms with broadcast semantics. `GET /api/share` SHALL
return the CALLER's capability (`{input, all, panes, view_panes}` — shape unchanged),
resolved for a guest from ITS OWN link scope. `gtmux share status --json` SHALL carry
each guest's `view_panes`, `panes`, and `expires_at` additively, and never a bare token.

#### Scenario: Two links, two scopes

- **WHEN** the host mints link A with `--view %1 --type %1` and link B with `--view %2`
- **THEN** A sees and may type into only `%1`, B sees only `%2` and may type nowhere,
  and each link's `GET /api/share` reports its own capability

#### Scenario: A guest is blocked until consent AND its own allowlist

- **WHEN** a guest token `POST`s `/api/send` while consent is off, or for a pane not on
  that link's input allowlist (even if another link allows it)
- **THEN** the send is refused (`403`) and the terminal is not touched

#### Scenario: Legacy global edits keep their meaning

- **WHEN** the host runs the legacy `gtmux share add %3` with two existing links
- **THEN** `%3` lands on BOTH links' input (and view) allowlists and on the template
  for future links — the pre-per-link behavior, byte-for-byte as a guest observes it

#### Scenario: Migration copies the global lists once

- **WHEN** serve starts with legacy guest links (no per-link scope) and a non-empty
  global allowlist
- **THEN** each such link receives a one-time copy of the global lists and behaves
  exactly as before the upgrade

#### Scenario: Allowing input implies view; removing view removes input (per link)

- **WHEN** the host grants a link input on a pane, then later removes that pane from the
  SAME link's view list
- **THEN** granting input marked it viewable, and removing view drops its input too —
  that guest can never type into a pane it cannot see

#### Scenario: The owner keeps full input

- **WHEN** the master token or a paired device reads or sends to any pane
- **THEN** it is unrestricted, regardless of consent or any link's allowlists

#### Scenario: A share link is revoked on its own

- **WHEN** the host revokes one guest share link
- **THEN** exactly that link's token stops working; other guests and the owner's own
  devices are unaffected

### Requirement: Guest read access is scoped to the view allowlist

For a `guest`-scope caller, the server SHALL filter every read surface to THAT LINK's
view allowlist: `GET /api/agents` SHALL return only the rows for panes it may view,
`GET /api/pane` SHALL refuse (`403`) a pane it may not view, and the SSE agent stream
and the web terminal mirror SHALL likewise expose only that link's viewable panes.
`full`-scope callers (master token, paired devices) SHALL be unfiltered. The filter is
server-side and authoritative.

#### Scenario: A fresh guest sees nothing

- **WHEN** a guest opens a newly minted share link whose view allowlist is empty
- **THEN** `GET /api/agents` returns an empty radar, `GET /api/pane` refuses every pane,
  and the web page shows no sessions to view

#### Scenario: Two guests read different radars

- **WHEN** link A may view `%A` and link B may view `%B`, and each reads `/api/agents`
- **THEN** A's radar contains only `%A`'s row and B's only `%B`'s — neither sees the
  other's grant

#### Scenario: The owner's read is unfiltered

- **WHEN** the master token or a paired device reads `/api/agents` or `/api/pane`
- **THEN** the full radar and any pane's text are returned, unaffected by any guest view
  allowlist

## ADDED Requirements

### Requirement: Share links may expire

A guest link SHALL support an optional expiry: minted with `--expires <duration>` (or
edited via `share set`), its token SHALL stop authenticating once the expiry passes —
rejected exactly like a revoked token (`401`) — with no background sweeper required.
The default SHALL be no expiry (revocation stays manual). `share status` SHALL show
each link's remaining validity; an expired link SHALL be labelled expired.

#### Scenario: An expired link stops authenticating

- **WHEN** a link minted with `--expires 24h` is used after 24 hours
- **THEN** every request with its token is rejected `401`, as if revoked

#### Scenario: No expiry by default

- **WHEN** a link is minted without `--expires`
- **THEN** it never expires; only revocation ends it

### Requirement: Pairing is one flow with three media

The system SHALL offer ONE owner-pairing flow rendered in three media from a single
short-lived enroll code: a QR (for the phone app), an `https://<host>/#c=<code>` URL
(for a browser), and a one-line `gtmux attach '<url>#c=<code>'` command (for another
computer's terminal). The `gtmux pair` command SHALL provide it: bare `gtmux pair`
mints a code and prints all three; `pair list` lists paired (owner) devices; `pair
revoke <id>` revokes one. `gtmux devices` SHALL remain as an alias of the roster
surface. All three media redeem into the SAME roster with `full` scope; guests never
appear under `pair list` (they live under `gtmux share status`).

#### Scenario: One code, three doors

- **WHEN** the owner runs `gtmux pair`
- **THEN** one enroll code is minted and printed as a QR, a browser URL, and an attach
  one-liner — any ONE of which can be redeemed once for an owner device token

#### Scenario: Guests never show as paired devices

- **WHEN** the owner runs `gtmux pair list` with guest links outstanding
- **THEN** only owner-scope devices are listed; the guests appear under
  `gtmux share status` instead
