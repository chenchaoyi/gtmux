# Browser Mirror Specification

## Purpose

Let a person on any computer watch a Mac's tmux agent sessions in a plain web
browser, with zero install — a view-only mirror of the agent radar and live panes,
served by `gtmux serve` / `gtmux tunnel` and reachable over LAN or the hosted
tunnel. Pairing is by a one-time link (from the serve/tunnel banner or handed off
from an already-paired phone), never the master token in a URL.
## Requirements
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

### Requirement: Desktop "workbench" mode

On a wide screen the served UI SHALL offer a "workbench" mode: a left session/agent
rail plus a freeform board of draggable, resizable pane tiles (multiple panes visible
at once), with layout presets, snap-to-grid, a ⌘K command palette, and an option to
auto-surface a pane the moment it starts waiting on the user. It remains VIEW-ONLY and
uses the same authed `/api/pane` + `/api/transcript` data as the single-pane mirror.

#### Scenario: Arrange multiple panes

- **WHEN** the user is on a wide screen and adds panes to the workbench board
- **THEN** each pane renders live and view-only, and can be dragged/resized/arranged,
  with the arrangement applied via presets or manual placement

### Requirement: Radar parity with the native surfaces

The browser radar SHALL reflect the same status vocabulary as the CLI/menu-bar: it
SHALL show sensed non-tmux sessions as a `source:"native"` "Elsewhere" category and
SHALL render the background-running idle modifier on an idle row whose settled turn
left in-flight background work.

#### Scenario: Native + background-running shown

- **WHEN** `/api/agents` includes a `source:"native"` row and/or an idle row carrying
  the background-running marker
- **THEN** the browser radar surfaces the "Elsewhere" category and the background
  modifier, matching the menu-bar/mobile presentation

### Requirement: Web UI served by `gtmux serve` — view, plus consented per-pane input

`gtmux serve` SHALL serve a self-contained web UI at `GET /`, embedded in the binary
(`//go:embed`, no build step, offline-safe, cgo-free) and served same-origin as the
existing `/api/*`. The UI SHALL present the agent radar and a pane mirror, using the
shared status language (color+shape+glyph; section order waiting→working→idle→running)
identical to the other surfaces. The UI is READ-ONLY BY DEFAULT. It MAY additionally
expose a terminal-input affordance, but ONLY for panes the caller is authorized to type
into, which it SHALL learn from `GET /api/share` (`{input, panes}` for the caller) and
NEVER assume: no input control is shown for a pane the server does not authorize, and
any typing goes through `POST /api/send`, whose server-side gate is authoritative. A
guest whose host has not consented, or a disallowed pane, shows no input control.

The capability SHALL also be TRANSPARENT (design-round-2026-07, WEB §11): every
focused pane / workbench tile head states its input capability explicitly — a cyan
`⌨ 可输入` chip when the caller may type, a grey `👁 只读` chip when not — and a
read-only pane shows a one-line "host 未授予此 pane 的输入权限" explanation instead
of an empty or missing input box. The top bar SHALL name the caller's identity
(owner = 全权, guest = 协作视图), resolved from `GET /api/share` (`all:true` ⇒
owner). On a typable waiting pane the `1/2/3` structured options SHALL be live —
one click sends the bare digit (no Enter) via `POST /api/send`, matching the phone's
ApprovalCard; on a view-only pane they stay inert with the reply-elsewhere hint.

#### Scenario: Browser loads the web UI

- **WHEN** a browser requests `GET /` from a running `gtmux serve`
- **THEN** the embedded web UI is returned and renders the agent radar after pairing

#### Scenario: Input shown only for authorized panes

- **WHEN** the UI has fetched `GET /api/share` for the caller
- **THEN** an input affordance is shown ONLY for the panes it lists; other panes stay
  view-only, and a guest with input off sees no input control anywhere

#### Scenario: The server gate, not the UI, is authoritative

- **WHEN** a guest `POST`s `/api/send` for a pane not in its authorized set
- **THEN** the send is refused server-side regardless of the UI state

#### Scenario: Capability is stated, not implied

- **WHEN** a caller focuses a pane (or has it on the workbench board)
- **THEN** its head shows `⌨ 可输入` (cyan) or `👁 只读` (grey) per the caller's
  `/api/share` capability, and a read-only term view carries the one-line
  "host 未授予" explanation rather than a blank where the input box would be

#### Scenario: Live 1/2/3 on a typable waiting pane

- **WHEN** a waiting pane the caller may type into shows its structured options
- **THEN** clicking an option sends that digit (no Enter) through `POST /api/send`,
  while a view-only caller sees the same options inert with the reply-elsewhere hint

