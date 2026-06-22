# Remote Access Specification

## Purpose

Expose the agent radar to a remote consumer (the mobile app) over a VPN/tunnel as
a read-only HTTP+SSE API, so you can see your agents and jump to a pane from your
phone without opening any write surface on the Mac.

## Requirements

### Requirement: Read-only HTTP API

The system SHALL, via `gtmux serve`, expose `GET /api/health`,
`GET /api/agents` (byte-identical to `agents --json`), `GET /api/pane?id=%N`
(read-only `capture-pane -p`), and `POST /api/focus?id=%N` (local pane select,
no input injection). It SHALL NOT expose any endpoint that writes to a terminal
or runs a command (read-only MVP).

#### Scenario: Agents match the CLI

- **WHEN** a client GETs `/api/agents`
- **THEN** the response is the same shape as `gtmux agents --json` (empty array
  when no tmux server)

#### Scenario: Focus selects only

- **WHEN** a client POSTs `/api/focus?id=%12`
- **THEN** the pane is selected locally and its tab brought forward; no input is
  injected

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
exposing an inbound port. The tunnel client runs only on the Mac; the phone app
is unchanged (it still pairs to a `{url, token}`). Cloudflare is the default
(no VPS, no account for a quick tunnel); frp is documented for the China / own-VPS
path. The command SHALL reuse the persistent serve token (so pairing matches a
running `gtmux serve`), start the read-only radar in-process when one is not
already up, and print the public URL plus a scannable pairing QR. It SHALL warn
that a public URL makes the bearer token the sole gate to the read-only radar.

#### Scenario: Quick tunnel from anywhere

- **WHEN** the user runs `gtmux tunnel` with `cloudflared` available
- **THEN** an outbound Cloudflare quick tunnel comes up, the command prints a
  public `https://*.trycloudflare.com` URL + the serve token + a pairing QR, and
  the phone (on any network) pairs and sees the live radar

#### Scenario: Token still gates a public URL

- **WHEN** the radar is reachable over a public tunnel URL
- **THEN** every `/api/*` route still requires the bearer token (no token → 401),
  unchanged from the LAN/VPN case

#### Scenario: Tunnel client missing

- **WHEN** `cloudflared` is not installed
- **THEN** the command offers to `brew install cloudflared` (with confirmation)
  and otherwise points at the manual install, rather than failing opaquely
