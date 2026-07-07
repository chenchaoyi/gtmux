## MODIFIED Requirements

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
