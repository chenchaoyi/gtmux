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
`/api/send` SHALL accept either an allow-listed named control key or literal text
(`send-keys -l`, optionally + Enter), and is gated by the same bearer token as the
rest (no separate authorization) — so the token also gates terminal input.

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

- **WHEN** a client GETs `/api/options` for a pane waiting on a numbered prompt
- **THEN** the response lists the `{n,label}` choices the parser found

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
exposing an inbound port. The tunnel client (`cloudflared`) runs only on the Mac;
the phone app is unchanged (it still pairs to a `{url, token}`), so the transport
never affects the app or its App Store availability. The command SHALL reuse the
persistent serve token (so pairing matches a running `gtmux serve`), start the
read-only radar in-process when one is not already up, print the public URL plus a
scannable pairing QR, and offer to `brew install cloudflared` when it is missing.
It SHALL warn that a public URL makes the bearer token the sole gate to the
read-only radar.

#### Scenario: Token still gates a public URL

- **WHEN** the radar is reachable over a public tunnel URL
- **THEN** every `/api/*` route still requires the bearer token (no token → 401),
  unchanged from the LAN/VPN case

#### Scenario: Tunnel client missing

- **WHEN** `cloudflared` is not installed
- **THEN** the command offers to `brew install cloudflared` (with confirmation)
  and otherwise points at the manual install, rather than failing opaquely

### Requirement: Hosted stable address by default; quick is opt-in

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
