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
for the live view — it binds the interface but does NOT itself tunnel. Push (see
push-notifications) arrives independently. A no-VPN hosted tunnel is a future
increment with its own security review.

#### Scenario: Same network or routable tunnel

- **WHEN** the phone shares a network with the Mac (same Wi-Fi, or a routable
  mesh VPN such as Tailscale)
- **THEN** the app pairs to the Mac's reachable address and the live view works

#### Scenario: Different networks, no tunnel

- **WHEN** the phone cannot reach the Mac (e.g. Mac at the office, phone at home,
  no routable VPN)
- **THEN** the live view is unavailable (push alerts still arrive); enabling it
  from anywhere is a deliberate future "remote-access tunnel" increment with its
  own security review
