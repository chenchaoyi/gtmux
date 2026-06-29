# Browser Mirror Specification

## Purpose

Let a person on any computer watch a Mac's tmux agent sessions in a plain web
browser, with zero install — a view-only mirror of the agent radar and live panes,
served by `gtmux serve` / `gtmux tunnel` and reachable over LAN or the hosted
tunnel. Pairing is by a one-time link (from the serve/tunnel banner or handed off
from an already-paired phone), never the master token in a URL.

## Requirements

### Requirement: View-only web UI served by `gtmux serve`

`gtmux serve` SHALL serve a self-contained, view-only web UI at `GET /`, embedded
in the binary (`//go:embed`, no build step, offline-safe, cgo-free) and served
same-origin as the existing `/api/*`. The UI SHALL present the agent radar and a
pane mirror, using the shared status language (color+shape+glyph; section order
waiting→working→idle→running) identical to the other surfaces. The UI SHALL expose
NO input, send-keys, or focus affordances in v1.

#### Scenario: Browser loads the web UI

- **WHEN** a browser requests `GET /` from a running `gtmux serve`
- **THEN** the embedded web UI is returned and renders the agent radar after pairing

#### Scenario: No write affordances

- **WHEN** the web UI is displayed
- **THEN** there is no control to type into, send keys to, or focus a pane (view-only)

### Requirement: Browser pairing via a one-time enroll code

The web UI SHALL authenticate by redeeming a short-lived, single-use enroll code
into a per-device token; the master token SHALL NOT be required or carried in a
URL. The code SHALL be read from the URL fragment (`/#c=<code>`), redeemed via the
existing `POST /api/enroll`, the resulting device token stored client-side
(cookie/localStorage), and the code stripped from the address bar after redemption.

#### Scenario: Pairing link authenticates the browser

- **WHEN** a browser opens `/#c=<valid-code>`
- **THEN** the page redeems the code via `/api/enroll`, stores the returned device
  token, removes the `#c=` fragment, and shows the radar

#### Scenario: Expired or invalid code

- **WHEN** a browser opens `/#c=<expired-or-unknown-code>`
- **THEN** the page shows a "get a fresh link" message and does not load agents

### Requirement: Pairing code from the serve banner or a phone handoff

A pairing code SHALL be obtainable two ways: (a) the banner of EITHER `gtmux serve`
(LAN) or `gtmux tunnel` (any network) SHALL print the browser URL(s) plus a
one-time pairing link; and (b) an already-paired phone SHALL be able to mint a code
(via the authenticated `POST /api/enroll/mint`) and share a pairing link so the
viewer can continue on a computer ("handoff").

#### Scenario: Serve banner advertises the browser (LAN)

- **WHEN** `gtmux serve` starts
- **THEN** its banner prints the reachable LAN browser URL(s) and a one-time
  pairing link

#### Scenario: Tunnel banner advertises the browser (any network)

- **WHEN** `gtmux tunnel` starts
- **THEN** its banner prints the public HTTPS browser URL (`https://gtmux-<id>.ccy.dev/`)
  and a one-time pairing link, reachable from any network

#### Scenario: Phone hands off to a computer

- **WHEN** a paired phone invokes "open on computer"
- **THEN** the phone mints a fresh code via `/api/enroll/mint`, builds a pairing
  link to the public/LAN URL, and offers it via the share sheet
- **AND WHEN** that link is opened in a computer browser
- **THEN** the browser pairs and shows the same agents the phone was watching

### Requirement: Live pane mirror that fits the browser window

Selecting a session SHALL render that pane's live screen with xterm.js, updated by
polling `GET /api/pane`, and SHALL refit the terminal to the browser window on
resize. Resizing SHALL change only the VIEW; it SHALL NOT resize the source Mac's
tmux pane.

#### Scenario: Live mirror updates

- **WHEN** a session is selected and its pane content changes on the source Mac
- **THEN** the browser mirror reflects the new content within the poll interval

#### Scenario: Window resize refits the view only

- **WHEN** the browser window is resized
- **THEN** the terminal refits to the new size and the source Mac's pane width is
  unchanged

### Requirement: Chat (对话) mode mirrors the transcript

The pane view SHALL offer a 对话/终端 (chat/terminal) switch; the chat mode SHALL
render the pane's parsed transcript (see `chat-transcript`) by polling
`GET /api/transcript` — a user-prompt bubble followed by the reply's `segments` as
separate speech bubbles with the interleaved tool steps as collapsible groups,
mirroring the phone's chat view. It stays view-only (no input).

#### Scenario: Switch to chat mode

- **WHEN** a session is selected and the user switches to 对话/chat mode
- **THEN** the browser renders the parsed transcript as a conversation (prompt
  bubble, segmented reply bubbles, collapsible steps) and keeps it fresh by polling
  `/api/transcript`

#### Scenario: Chat mode is view-only

- **WHEN** the chat mode is displayed
- **THEN** there is no control to type, send, or focus (view-only, like the pane mirror)

### Requirement: Reachable over LAN and tunnel

The web UI SHALL be reachable on the LAN (`http://<ip>:<port>/`) and remotely via
the existing `gtmux tunnel` public HTTPS hostname (`https://gtmux-<id>.ccy.dev/`),
with no change to the `/api/*` contract.

#### Scenario: Opened over the tunnel

- **WHEN** `gtmux tunnel` is running and a browser opens the public HTTPS URL with a
  valid pairing link
- **THEN** the web UI pairs and mirrors panes over the tunnel
