# Remote Access Specification

## Purpose

Expose the agent radar to a remote consumer (the mobile app) over a VPN/tunnel as
a read-only HTTP+SSE API, so you can see your agents and jump to a pane from your
phone without opening any write surface on the Mac.
## Requirements
### Requirement: HTTP API (read + terminal input)

The system SHALL, via `gtmux serve`, expose `GET /api/health`,
`GET /api/agents` (byte-identical to `agents --json`), `GET /api/pane?id=%N`
(`capture-pane -e`, ANSI color), `POST /api/focus?id=%N` (local pane select, no
input), and `POST /api/send` (type into a pane via `tmux send-keys` — a WRITE).
`/api/send` SHALL accept either an allow-listed named control key or literal text, and
is gated by the same bearer token as the rest (no separate authorization) — so the token
also gates terminal input. SINGLE-LINE text SHALL be delivered as literal KEYSTROKES
(`send-keys -l`) so an agent's numbered menu commits on the digit alone; MULTI-LINE text
SHALL ride the tmux paste buffer (bracketed) so its newlines don't reach the TUI as bare
Returns that submit each line separately. When `enter` is set, submission is a separate,
confirmed step.

#### Scenario: Agents match the CLI

- **WHEN** a client GETs `/api/agents`
- **THEN** the response is the same shape as `gtmux agents --json` (empty array
  when no tmux server)

#### Scenario: Focus selects only

- **WHEN** a client POSTs `/api/focus?id=%12`
- **THEN** the pane is selected locally and its tab brought forward; no input is
  injected

#### Scenario: Send types into the pane

- **WHEN** a client POSTs `/api/send` with `{id, text, enter}` or `{id, key}`
- **THEN** the text (literal) or the allow-listed control key is sent to that pane
  via `tmux send-keys`; a disallowed key or missing pane returns an error

#### Scenario: A numbered-menu digit selects the option (keystroke, not paste)

- **WHEN** a client POSTs `/api/send` with a single-line `{id, text}` (e.g. `"1"`) and no
  Enter, answering an agent's numbered menu
- **THEN** the digit is sent as a literal keystroke (`send-keys -l`) and the menu commits
  that choice — a bracketed paste of the digit would be inserted as text and select
  nothing

### Requirement: Pane capture preserves a bottom-anchored cursor

`GET /api/pane` SHALL capture with `capture-pane -e -p -S -2000` (visible screen +
up to 2000 scrollback lines, ANSI SGR kept) and SHALL NOT right-trim trailing blank
rows, so the cursor offset anchors to the true bottom. The response MAY include an
optional `cursor{x, up, visible}` where `up` is rows up from the LAST captured line
(`pane_height-1-cursor_y`, `0` = bottom row) — letting a client place a cursor cell
without knowing the capture's top. The field is omitted when no cursor resolves.

#### Scenario: Cursor anchors to the bottom

- **WHEN** a client GETs `/api/pane` for a pane with a visible cursor
- **THEN** the response carries `cursor.up` measured up from the last captured line
  and `cursor.x` as the column, and trailing blank rows are not stripped

### Requirement: Read endpoints for chat, choices, and appearance

The system SHALL expose read-only `GET /api/transcript?id=%N` (the pane's parsed
chat history — see `chat-transcript`), `GET /api/options?id=%N` (a waiting pane's
parsed `1/2/3` choices, same parser as the menu-bar/CLI), and `GET /api/theme`
(the host terminal's resolved appearance — see `terminal-theme`). `/api/transcript`
returns `503` when its dependency is not wired and `[]` when the pane has no
session; `/api/options` returns `{"options":[]}` when nothing parses.

#### Scenario: Transcript when no session

- **WHEN** a client GETs `/api/transcript` for a pane with no resumable session
- **THEN** the server returns an empty list (not an error)

#### Scenario: Options parse a waiting prompt

- **WHEN** a client GETs `/api/options` for a pane the hook marks as waiting on a
  numbered prompt
- **THEN** the response lists the `{n,label}` choices the parser found

#### Scenario: Options are gated on the hook waiting state

- **WHEN** a client GETs `/api/options` for a pane that is NOT hook-waiting, even if
  its screen text looks like a numbered menu
- **THEN** the response is `{"options":[]}` — options are only parsed for a
  hook-confirmed waiting pane, never inferred from arbitrary output

### Requirement: Per-device enrollment so the master token isn't shared

The system SHALL let a trusted surface mint a short-lived single-use enroll code
(`POST /api/enroll/mint`) that a new device redeems for its own per-device token
(`POST /api/enroll`, the only authenticated-exempt `/api/*` route besides
`/api/health` — the code itself is the credential), and SHALL let the roster be
listed (`GET /api/devices`, no tokens) and revoked (`POST /api/devices/revoke`), so
a phone/browser never carries the master token and a lost device can be cut off.

#### Scenario: Redeem an enroll code

- **WHEN** a device POSTs a valid enroll code to `/api/enroll`
- **THEN** it receives its own `{token, deviceId}`; an expired/unknown code returns 401

#### Scenario: Revoke a device

- **WHEN** the master surface POSTs a device id to `/api/devices/revoke`
- **THEN** that device's token stops working immediately

### Requirement: Bearer auth, intranet bind

The system SHALL guard every `/api/*` route except `/api/health` with a constant-
time Bearer token check, persist the token `0600` at
`~/.config/gtmux/serve-token` (or accept `--token`), and bind an intranet/VPN
interface (default `0.0.0.0`), never the public internet.

#### Scenario: Bad token rejected

- **WHEN** a request to a guarded route presents a missing/incorrect token
- **THEN** the server responds 401

### Requirement: Live updates via SSE

The system SHALL provide `GET /api/events` (SSE) that emits `agents{rev}` when the
set/status changes (refetch trigger), `alert{pane,kind,…}` on a waiting/done
transition, and `ping` heartbeats. `/api/agents` stays the only data payload.

#### Scenario: Change signals a refetch

- **WHEN** an agent's status changes
- **THEN** the server emits an `agents` SSE event and the client refetches
  `/api/agents`

### Requirement: Versioned contract

The system SHALL treat `api/contract.md` as the versioned source of truth (`v0`);
breaking changes bump the version and route prefix.

#### Scenario: Contract change

- **WHEN** a change breaks the v0 shape
- **THEN** the version and prefix are bumped rather than silently changed

### Requirement: Reachability is the consumer's network responsibility

The system SHALL require the consumer to provide network reachability to the Mac
for the live view — the radar server binds the interface but does NOT itself
tunnel. Push (see push-notifications) arrives independently. Reachability may come
from the same network, a mesh VPN (Tailscale), or an outbound tunnel (see the
tunnel requirement below); the transport never reaches the phone app, which only
ever holds a `{url, token}` pairing.

#### Scenario: Same network or routable tunnel

- **WHEN** the phone shares a network with the Mac (same Wi-Fi, or a routable
  mesh VPN such as Tailscale)
- **THEN** the app pairs to the Mac's reachable address and the live view works

#### Scenario: Different networks, no tunnel

- **WHEN** the phone cannot reach the Mac (e.g. Mac at the office, phone at home)
  and no VPN or tunnel is set up
- **THEN** the live view is unavailable (push alerts still arrive); `gtmux tunnel`
  enables it from anywhere

### Requirement: Outbound tunnel for no-VPN remote access

The system SHALL provide `gtmux tunnel` — a Mac-side, outbound reverse tunnel that
makes the read-only radar reachable from anywhere without a VPN app and without
exposing an inbound port. The tunnel transport is provided by a **pluggable
provider**: `cloudflare` (default; `cloudflared`) or `self` (a user-hosted
WebSocket-over-443 backend on the user's own VPS + domain). The tunnel client runs
only on the Mac; the phone app is unchanged (it still pairs to a `{url, token}`), so
the transport never affects the app or its App Store availability. Regardless of
provider, the command SHALL reuse the persistent serve token, start the read-only
radar in-process when one is not already up, print the public URL plus a scannable
pairing QR, and offer to install the selected provider's client when it is missing.
It SHALL warn that a public URL makes the bearer token the sole gate.

#### Scenario: Token still gates a public URL

- **WHEN** the radar is reachable over a public tunnel URL (any provider)
- **THEN** every `/api/*` route still requires the bearer token (no token → 401),
  unchanged from the LAN/VPN case

#### Scenario: Tunnel client missing

- **WHEN** the selected provider's client (`cloudflared` or `chisel`) is not installed
- **THEN** the command offers to install it (with confirmation) and otherwise points
  at the manual install, rather than failing opaquely

#### Scenario: Default provider is Cloudflare

- **WHEN** `gtmux tunnel` is run with no provider override
- **THEN** it uses the Cloudflare backend exactly as before (hosted stable address),
  so existing setups and pairings are unaffected

#### Scenario: Self-hosted provider on a hostile network

- **WHEN** the user selects the self-hosted backend (`gtmux tunnel --backend self` or
  `GTMUX_TUNNEL_BACKEND=self`) with their server configured (`GTMUX_SELFTUNNEL_URL` +
  `GTMUX_SELFTUNNEL_SECRET`)
- **THEN** the Mac dials out over 443 to the user's own domain and prints that domain
  as the pairing URL, so the phone pairs the same way
- **AND** when the self-hosted config is absent, the command explains what to set
  rather than crashing

#### Scenario: Selecting self is independent of the Cloudflare edge

- **WHEN** the Cloudflare edge is blocked on the current network (the hosted tunnel
  can't register)
- **THEN** the self-hosted provider still works if the user's VPS is reachable on 443
  (its traffic is indistinguishable from ordinary HTTPS to the user's own domain)

#### Scenario: Switching remote mode tears down the ACTIVE backend

- **WHEN** the always-on tunnel is running on the self-hosted (Direct) backend and the
  user turns remote access Off (or down to Wi-Fi) — via `gtmux serve --unservice` /
  `--service` or the menu-bar Off/Wi-Fi picker
- **THEN** the self-hosted tunnel agent (`com.gtmux.selftunnel`) is unloaded + removed
  along with the serve and Cloudflare agents, so the derived mode actually leaves
  Anywhere (it does not read `.anywhere` because a backend agent was left behind)

The system SHALL, by default, give each Mac a STABLE hosted address so the phone
pairs ONCE and keeps reaching the Mac across restarts. A control-plane service
(`tunnel-worker/`, a Cloudflare Worker) SHALL idempotently provision a Cloudflare
*named* tunnel per Mac — keyed by a persisted random `deviceId` — point its ingress
at the local serve port, create a single-level DNS host (so the zone's free
Universal cert covers it; a deeper host would need paid certs), and return the
connector token the Mac runs `cloudflared` with. `gtmux tunnel --quick` SHALL
instead use an account-less Cloudflare quick tunnel whose URL rotates each run. The
hosted registration gate ships in the binary (a soft anti-abuse speed bump, not a
real secret) and SHALL be overridable, with the control-plane URL, via environment
variables for self-hosting.

#### Scenario: Stable address, pair once

- **WHEN** the user runs `gtmux tunnel` (hosted default) on a configured build
- **THEN** the control plane returns the same stable `gtmux-<id>.ccy.dev` address
  for that Mac on every run, cloudflared connects with the returned token, and the
  phone pairs once and keeps working across restarts

#### Scenario: Ephemeral quick tunnel

- **WHEN** the user runs `gtmux tunnel --quick`
- **THEN** an account-less `https://*.trycloudflare.com` tunnel comes up whose URL
  changes each run, with the same read-only + token guarantees

#### Scenario: Hosted not configured in this build

- **WHEN** hosted mode is unconfigured (no registration gate baked in or set)
- **THEN** `gtmux tunnel` does not fail opaquely — it tells the user to use
  `--quick` or set the override env vars

#### Scenario: Self-hosted control plane

- **WHEN** a self-hoster sets the control-plane URL + registration override env vars
- **THEN** `gtmux tunnel` provisions against their own Worker instead of gtmux's

### Requirement: Paid "Direct" tier via redeemable code

The system SHALL provide a paid "Direct" tier layered on the self-hosted (443)
backend: `gtmux tunnel --redeem <code>` exchanges a purchased code at the tunnel
provisioner (`POST /direct/redeem`, validated against a `DIRECT_CODES` store) for the
Direct server URL + shared secret, which are persisted to the local self-tunnel config
so subsequent `gtmux tunnel --backend self` runs need no manual `GTMUX_SELFTUNNEL_URL`/
`SECRET`. The in-process Chisel client (no external binary) carries the transport.

#### Scenario: Redeem a Direct code

- **WHEN** the user runs `gtmux tunnel --redeem <code>` with a valid code
- **THEN** the provisioner returns the Direct URL + secret, they are written to the
  self-tunnel config, and later `--backend self` runs connect with no manual env

#### Scenario: Invalid or spent code

- **WHEN** the redeemed code is unknown/expired
- **THEN** the command reports it clearly and writes no config (no opaque failure)

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

### Requirement: WebSocket attach endpoint

The serve contract SHALL include `GET /api/attach?id=%N` — a WebSocket endpoint that
bridges a tmux pane's PTY to the caller. It SHALL be authenticated and scope-gated: an
owner (master/device token) may attach any pane; a `guest` token may attach ONLY a
view-allowed pane, and the server SHALL refuse the upgrade otherwise. The bridge SHALL
use binary frames with a one-byte opcode (client→server `INPUT`/`RESIZE`/`PAUSE`/
`RESUME`, server→client `OUTPUT` carrying raw PTY bytes), DROP write frames
(`INPUT`/`RESIZE`) for a pane the caller may not type into, and bound its buffering with
client-driven `PAUSE`/`RESUME` flow control. Scope enforcement is server-side and
authoritative; a client flag never widens it.

#### Scenario: Guest upgrade refused for a non-viewable pane

- **WHEN** a guest opens `/api/attach?id=%N` for a pane not on its view allowlist
- **THEN** the server refuses the WebSocket upgrade and spawns no PTY

#### Scenario: View-only input is dropped

- **WHEN** a guest attached to a view-only pane sends an `INPUT` frame
- **THEN** the server does not write it to the pane

### Requirement: The CLI is a first-class client of the serve contract

The gtmux CLI (`gtmux attach`) SHALL be a first-class client of the `gtmux serve`
contract, alongside the web page and the mobile app, using the SAME token-scope model:
a device/master token attaches as an owner (full), a `guest` token attaches
scope-restricted (view-only panes are read-only). The server enforces scope identically
regardless of client surface.

#### Scenario: A terminal client with a guest token is restricted

- **WHEN** `gtmux attach https://host/#g=<token> %N` connects with a guest token
- **THEN** it is restricted exactly as a guest browser/app — refused a non-viewable pane, read-only on a view-only one

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

### Requirement: Owner devices manage sharing; the device roster and door stay Mac-scoped

The system SHALL authorize the remote-management surface by caller scope so that an
owner device (a paired phone / browser / terminal — `full` scope, like the master
token) can MANAGE SHARING remotely, while the device roster and the remote-access
door remain restricted:

- The SHARE-management endpoints — `POST /api/share/config` (consent + the global
  lists), `POST /api/share/new`, `POST /api/share/set` — SHALL accept any `full`
  caller (master OR owner device) and SHALL refuse a `guest` (`403`).
- `GET /api/devices` SHALL be restricted to `full` callers (master or owner
  device) and SHALL refuse a `guest` (`403`).
- `POST /api/devices/revoke` SHALL honor the caller's scope: a master MAY revoke
  ANY roster entry; an owner device MAY revoke ONLY a `guest` entry (a share
  link) and SHALL be refused (`403`) when the target is a paired device; a guest
  SHALL be refused entirely.
- Toggling the remote-access door (starting/stopping serve or the tunnel) SHALL
  remain a local Mac operation with no remote endpoint.
- `GET /api/share/link?id=<id>` SHALL re-hand an existing guest link's TOKEN to a
  `full` caller (master or owner device) and SHALL refuse a `guest` (`403`) and an
  unknown id (`404`), so a host who didn't copy a link at mint time can copy it
  again (the CLI `gtmux share link <id>` and the menu-bar row's copy button both use
  it; the token rides the `#g=` URL fragment, never a bare field).

`GET /api/share` (the caller's own capability) SHALL remain available to any
authenticated scope, unchanged.

#### Scenario: An owner device manages a share link

- **WHEN** an owner device token calls `POST /api/share/new`, `POST /api/share/set`,
  or `POST /api/share/config`
- **THEN** the operation is authorized (as it would be for the master token)

#### Scenario: A guest cannot reach the management surface

- **WHEN** a `guest` token calls `GET /api/devices`, `POST /api/share/config`,
  `POST /api/share/new`, or `POST /api/share/set`
- **THEN** each is refused (`403`) — closing the prior unguarded device-list/revoke

#### Scenario: An owner may revoke a share link but not a paired device

- **WHEN** an owner device calls `POST /api/devices/revoke` for a `guest` entry
- **THEN** the share link is revoked
- **WHEN** the same owner device calls it for a paired (`device`) entry
- **THEN** it is refused (`403`), and only the master token (from the Mac) can
  revoke a paired device

#### Scenario: Re-hand an existing link's URL

- **WHEN** a `full` caller (master or owner device) calls `GET /api/share/link?id=<id>`
  for a live guest link
- **THEN** it returns that link's token (for rebuilding the `#g=` URL)
- **WHEN** a `guest` calls it, or the id is unknown
- **THEN** it is refused (`403`) or reported not-found (`404`) respectively

### Requirement: Revoke unregisters the device's push token; the token store is master-only

`POST /api/devices/revoke` SHALL, after removing a roster entry, unregister every push
token bound to that device id (so revoking a device stops its notifications in one step,
consistent with the scoped-revoke authorization already in place). The push-token
management endpoints SHALL be MASTER-only:

- `GET /api/push/tokens` SHALL return the registered tokens REDACTED (prefix only) to the
  master token, and refuse any non-master caller (`403`).
- `POST /api/push/forget {deviceId|orphans|all}` SHALL drop the selected tokens for the
  master token, and refuse any non-master caller (`403`).

`POST /api/push/register` / `POST /api/push/unregister` (a device managing its OWN token)
remain available to an authenticated device as before.

#### Scenario: Revoke drops the token

- **WHEN** the master (or an owner device, per the scoped-revoke rules) revokes a device
  that had a registered push token
- **THEN** the revoke also unregisters that device's push token

#### Scenario: Token store is master-only

- **WHEN** a device or guest token calls `GET /api/push/tokens` or `POST /api/push/forget`
- **THEN** it is refused (`403`), and only the master token (the Mac's own CLI) may
  inspect or clear the store

### Requirement: Pane grants are valid only for the tmux server they were made against

The system SHALL bind guest pane grants to the identity of the tmux server they were
granted against, and SHALL REFUSE those grants while a different tmux server is running,
until the owner grants again. This is required because a tmux pane id is unique only
within one tmux server lifetime: after a restart the ids are reassigned, so a stored id
addresses a different pane than the owner shared. Refusal SHALL apply to every guest path: attaching (before any PTY is spawned),
sending input, reading pane content, and the guest's agent list (which SHALL be empty
rather than risk revealing a pane the owner never shared). Grants carrying no recorded
server identity SHALL be treated as invalid. When no tmux server identity is available the
system SHALL NOT treat grants as invalid. The system SHALL report this state to the owner
so they can re-grant, rather than failing silently. The system SHALL NOT automatically
re-map grants onto a new server by session name, since a name can be reused or renamed and
an automatic re-map could grant access to the wrong session.

#### Scenario: Grants from a previous tmux server are refused

- **WHEN** a guest whose link was scoped before a reboot tries to view or type into a pane after restore
- **THEN** the request is refused and the guest is not shown any pane, because the stored pane ids can no longer be proven to mean what the owner shared

#### Scenario: Re-granting restores access

- **WHEN** the owner grants pane scope again against the running tmux server
- **THEN** the grants are bound to that server and the guest's access works normally

#### Scenario: The owner is told, not left guessing

- **WHEN** grants were made against a different tmux server
- **THEN** the share status reports the grants as stale and tells the owner to re-grant

### Requirement: Supervisor knowledge is served to owner clients

The server SHALL expose the supervisor's situation board and the severity-tagged event
ledger to owner clients, so a remote surface can present the supervisor's own assessment
and the fleet's history rather than only the present instant. The board SHALL be served
read-only, with the time it was last written, and SHALL report its absence as an ordinary
state rather than an error, since a supervisor that has written no board yet is normal.
The event ledger SHALL be filterable by a severity floor and bounded in length, because a
remote client must not have to download the whole log to show recent activity. Both
SHALL be refused to a guest caller: they carry the whole fleet and the supervisor's
private assessment, which are owner surfaces and never part of a shared scope. Both SHALL
remain available when no supervisor is running, reporting empty rather than failing.

#### Scenario: An owner client reads the supervisor's board

- **WHEN** an owner-scoped client requests the situation board
- **THEN** it receives the board's text and the time it was last written

#### Scenario: No board has been written

- **WHEN** the supervisor has never written a situation board
- **THEN** the request succeeds and reports that no board exists, rather than erroring

#### Scenario: Recent activity is bounded and filtered

- **WHEN** an owner-scoped client requests the event ledger at a severity floor
- **THEN** it receives only records at or above that severity, newest first, no more
  than the requested number

#### Scenario: A guest cannot read either

- **WHEN** a guest-scoped caller requests the board or the event ledger
- **THEN** the request is refused, exactly as for the digest and usage surfaces

### Requirement: A roster entry identifies the device it names

A paired device SHALL register under a name that identifies THAT device, using what the
device knows about itself (its form factor and OS version), and SHALL NOT prefix it with
the product's own name: inside gtmux's own roster a "gtmux" prefix carries no information
— nothing in that list is not a gtmux device — while pushing the identifying part out to
where a narrow row truncates it. The system SHALL NOT claim a device model it cannot
establish, since a confidently wrong model is worse than an honest general one. Surfaces
that display the roster SHALL strip a legacy product prefix from entries registered
before this rule, so an existing roster reads correctly without anyone re-pairing, and
SHALL never render an entry as blank.

#### Scenario: A phone pairs

- **WHEN** a phone enrolls into the roster
- **THEN** its entry names its form factor and OS version, with no product prefix

#### Scenario: An entry from before the rule

- **WHEN** the roster contains an entry registered with the old product prefix
- **THEN** every surface displays it without that prefix

#### Scenario: A device actually named after the product

- **WHEN** an entry's whole name is the product name
- **THEN** it is still displayed, rather than being stripped to an empty row

### Requirement: A message that was not submitted is reported as a failure

The system SHALL report a message it declined to submit as a FAILURE, never as success.
It pastes into a pane's input box and confirms the full message landed before submitting;
when it cannot confirm, it does not submit. Reporting success for a message that was never
submitted is worse than the refusal it hides: the message sits unsent in the box on the
host while every remote surface shows it as delivered, and the sender can only discover
this by inspecting the host directly. The time allowed for a paste to render before it is
judged incomplete SHALL scale with the size of the message, since a fixed budget makes a
large message fail for being large rather than for being wrong. A client receiving this
failure SHALL tell the user and SHALL preserve the message text so it can be retried
without retyping.

#### Scenario: A paste that could not be confirmed

- **WHEN** the input box does not settle on the full message
- **THEN** the message is not submitted AND the caller is told the send failed

#### Scenario: A large message is given time to render

- **WHEN** a long message is pasted
- **THEN** it is allowed proportionally more time to appear before being judged incomplete

#### Scenario: The sender keeps their text

- **WHEN** a send is reported as failed
- **THEN** the client shows the failure and keeps the text available to retry
