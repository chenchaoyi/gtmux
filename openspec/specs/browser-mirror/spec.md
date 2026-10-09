# Browser Mirror Specification

## Purpose

Let a person on any computer watch a Mac's tmux agent sessions in a plain web
browser, with zero install — a mirror of the agent radar and live panes with
input only where the owner or a share link grants it,
served by `gtmux serve` / `gtmux tunnel` and reachable over LAN or the hosted
tunnel when the network permits it. New owner pairing links use a one-time code
from the Mac; a guest can use a share link. The browser also accepts legacy token
fragments for compatibility. A phone-to-computer handoff action was removed in #512;
the archived browser-mirror proposal records the earlier design, not the current UI.

## Requirements

### Requirement: Browser access copy serves owners and guests

The browser access page SHALL explain that its code field accepts either an
owner pairing code or a guest share code. A rejected code SHALL prompt the
reader to check it or get a new one without assuming a share-link sender.
An empty chat view SHALL point to Terminal for the current screen without
misstating how the agent transcript is recorded.

#### Scenario: A Mac owner mistypes a pairing code

- **WHEN** enrollment refuses the code entered on the browser access page
- **THEN** the page gives a usable next step without telling the owner to ask
  another person to resend a share link

### Requirement: Browser pairing via a one-time enroll code

The web UI SHALL authenticate by redeeming a short-lived, single-use enroll code
into a per-device token; a newly generated owner pairing link SHALL carry a code,
not the master token. The browser SHALL read `/#c=<code>` for owner pairing and
`/#code=<code>` for share codes, redeem through `POST /api/enroll`, and store the
resulting token in localStorage. It SHALL remove the credential fragment before
starting redemption. Legacy `#g=` / `#t=` token fragments remain accepted and are
also removed from the address bar; this compatibility path is not code redemption.

#### Scenario: Pairing link authenticates the browser

- **WHEN** a browser opens `/#c=<valid-code>`
- **THEN** the page redeems the code via `/api/enroll`, stores the returned device
  token, removes the `#c=` fragment, and shows the radar

#### Scenario: Expired or invalid code

- **WHEN** a browser with no saved credential opens `/#c=<expired-or-unknown-code>`
- **THEN** the page explains that access is not set up, offers a code field and
  the Mac’s `gtmux pair` path, and does not load agents

### Requirement: Pairing code from the Mac

A browser owner SHALL be able to obtain a one-time pairing link with `gtmux pair`
on the Mac. Interactive `gtmux serve` and `gtmux tunnel` banners SHALL advertise
browser addresses, and print a code link when a code is available. A loaded
always-on tunnel is reused; if its URL file is missing, the command points to
`gtmux tunnel --status` instead of inventing an address. If tunnel code minting is
unavailable, its legacy QR may contain the master token, but its printed bare
browser URL does not by itself authenticate a new browser.

The phone's former “Open on computer” settings row was deliberately removed by
commit `61bf41e85669e9d5dcdf0c9fc46862eebeade853` (#512). An API client helper or an
archived checked task is not evidence that the current phone offers that action.

The `gtmux serve` banner SHALL print the token and the pairing link only when it prints
to a terminal. Written anywhere else, which in practice is the LaunchAgent's capture
(`logs/serve.stderr`), it SHALL print neither, SHALL NOT mint a code for it, and SHALL say where
the token lives and how to pair instead; serve SHALL also narrow its stdout and stderr
to their owner (0600) when they are plain files. Either credential gets a device full
control of the Mac, and that log is created world-readable under directories other
accounts can list: before this it held the token once per restart and a fresh pairing
code with each.

#### Scenario: Serve banner advertises the browser (LAN)

- **WHEN** `gtmux serve` starts in a terminal
- **THEN** its banner prints the reachable LAN browser URL(s), the token and a one-time
  pairing link

#### Scenario: Serve under launchd writes no credential to its log

- **WHEN** `gtmux serve` starts with its output going to a file
- **THEN** the file is made readable by its owner only, and the banner in it names
  neither the token nor a pairing link, pointing at `gtmux pair` instead

#### Scenario: Tunnel banner advertises the public browser address

- **WHEN** the selected tunnel starts and a pairing code is minted
- **THEN** its banner prints that tunnel’s public HTTPS browser URL (for hosted
  Standard, `https://gtmux-<id>.ccy.dev/`) and a one-time pairing link
- **AND** access still requires a working Mac-to-tunnel and browser-to-public-endpoint route

#### Scenario: Pair a new browser from the Mac

- **WHEN** the owner runs `gtmux pair` against a reachable local serve
- **THEN** it prints a one-time browser pairing link; opening it redeems the code
  for that browser’s own credential

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

#### Scenario: A phone-width window keeps the pane's controls on screen

- **WHEN** a pane is open in a window narrower than 800px
- **THEN** its top bar wraps instead of overflowing: back, title and server stay on the
  first row, the title truncating first, and the identity and input chips and the
  controls follow on the rows below; nothing in the bar lies outside the window and the
  page does not scroll sideways
- **AND** the appearance panel opens below the bar, not over it

### Requirement: Chat (对话) mode mirrors the transcript

The pane view SHALL offer a 对话/终端 (chat/terminal) switch; the chat mode SHALL
render the pane's parsed transcript (see `chat-transcript`) by polling
`GET /api/transcript` — a user-prompt bubble followed by the reply's `segments` as
separate speech bubbles with the interleaved tool steps as collapsible groups,
mirroring the phone's chat view. Chat SHALL have no free-text composer. Its
waiting card MAY send a parsed option's number when the caller is authorized, as
defined by the input-capability requirement below.

#### Scenario: Switch to chat mode

- **WHEN** a session is selected and the user switches to 对话/chat mode
- **THEN** the browser renders the parsed transcript as a conversation (prompt
  bubble, segmented reply bubbles, collapsible steps) and keeps it fresh by polling
  `/api/transcript`

#### Scenario: Free-text entry remains in Terminal

- **WHEN** the chat mode is displayed
- **THEN** there is no free-text input box in Chat; the caller switches to Terminal
  to type a message, while authorized structured-choice replies remain available in Chat

### Requirement: Reachable over LAN and tunnel

The web UI SHALL be reachable on the LAN (`http://<ip>:<port>/`) and remotely via
the existing `gtmux tunnel` public HTTPS hostname (`https://gtmux-<id>.ccy.dev/`),
with the same `/api/*` contract. LAN access requires a reachable bind address and
firewall route; remote access requires both sides to reach the tunnel. Direct may
serve under a `/p<port>/` path, which the browser SHALL preserve for API requests.

#### Scenario: Opened over the tunnel

- **WHEN** `gtmux tunnel` is running and a browser opens the public HTTPS URL with a
  valid pairing link
- **THEN** the web UI pairs and mirrors panes over the tunnel

### Requirement: Desktop "workbench" mode

On a wide screen the served UI SHALL offer a "workbench" mode: a left session/agent
rail plus a freeform board of draggable, resizable pane tiles (multiple panes visible
at once), with layout presets, snap-to-grid, a ⌘K command palette, and an option to
auto-surface a newly waiting pane when the next agent poll detects it (after the
initial status baseline). Layout changes affect
only this browser; input is available in panes authorized by `/api/share`. It uses
the same authenticated `/api/pane` + `/api/transcript` data as the single-pane mirror.

#### Scenario: Arrange multiple panes

- **WHEN** the user is on a wide screen and adds panes to the workbench board
- **THEN** each pane renders live and can be dragged/resized/arranged, with the
  arrangement applied via presets or manual placement; authorized panes may accept input

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
shared status language (color+shape+glyph; sections waiting→errored→working→idle→running,
where errored groups an idle turn that ended in failure)
identical to the other surfaces. The UI is READ-ONLY BY DEFAULT. It MAY additionally
expose a terminal-input affordance, but ONLY for panes the caller is authorized to type
into, which it SHALL learn from `GET /api/share` (`{input, panes}` for the caller) and
NEVER assume: no input control is shown for a pane the server does not authorize, and
any typing goes through `POST /api/send`, whose server-side gate is authoritative. A
guest whose host has not consented, or a disallowed pane, shows no input control.

The capability SHALL also be TRANSPARENT (design-round-2026-07, WEB §11): every
focused pane / workbench tile head states its input capability explicitly — a cyan
`⌨ 可输入` chip when the caller may type, a grey `👁 只读` chip when not — and a
read-only pane explains that input is not allowed instead
of an empty or missing input box. The top bar SHALL name the caller's identity
(owner = 全权, guest = 协作视图), resolved from `GET /api/share` (`all:true` ⇒
owner). On a typable waiting pane the parsed numbered options SHALL be live —
one click sends the bare digit (no Enter) via `POST /api/send`, matching the phone's
ApprovalCard; on a view-only pane they stay inert with the reply-elsewhere hint. Only
options `GET /api/options` actually returned SHALL be shown as numbered options: with
none (an open question, or the options request failed) the page SHALL draw no number
buttons, and SHALL say there are no numbered choices and where to answer instead.

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

#### Scenario: A send that does not complete keeps the text

- **WHEN** the composer's send is refused (any non-2xx other than 401, a 403 included) or gets no answer (the request fails)
- **THEN** the text goes back into the box if the box is still empty, a note says why (refused, or not confirmed: it may or may not have reached the Mac), and nothing is sent again by itself

#### Scenario: Capability is stated, not implied

- **WHEN** a caller focuses a pane (or has it on the workbench board)
- **THEN** its head shows `⌨ 可输入` (cyan) or `👁 只读` (grey) per the caller's
  `/api/share` capability, and a read-only term view carries the one-line
  input-not-allowed explanation rather than a blank where the input box would be

#### Scenario: Live parsed choices on a typable waiting pane

- **WHEN** a waiting pane the caller may type into shows its structured options
- **THEN** clicking an option sends that digit (no Enter) through `POST /api/send`,
  while a view-only caller sees the same options inert with the reply-elsewhere hint

#### Scenario: A waiting pane with no numbered choices gets no number buttons

- **WHEN** a pane is waiting and `GET /api/options` returns no options, or fails
- **THEN** the reply bar and the chat card show no numbered buttons, say there are no numbered choices and where to answer, and nothing can send a digit; the chat card's heading reads "waiting for your answer", not "approval"

### Requirement: A page that cannot show anything explains itself

When the browser cannot show sessions — it is not paired, or its link expired — the page
SHALL state WHICH of those it is, and SHALL give the exact step that resolves it. It SHALL
name only affordances that exist: an instruction referring to a control the product does
not have is worse than no instruction, because the reader spends their time looking for
it. It SHALL identify itself as gtmux, since a reader may arrive from a bookmark with no
other context. Commands SHALL be rendered as commands rather than as their markup source.
Like every other surface, it SHALL be bilingual.

#### Scenario: An unpaired browser

- **WHEN** the page is opened with no credential
- **THEN** it says the browser isn't paired, and gives the command that produces a link
  plus which of that command's outputs to open

#### Scenario: An expired link

- **WHEN** the page's credential is rejected as expired
- **THEN** it says the link expired — distinct from never having been paired — and how to
  mint a fresh one

#### Scenario: A guest arrives

- **WHEN** an unpaired reader holds a link someone shared with them
- **THEN** the page tells them such a link works without pairing

### Requirement: The pane view shows Codex's pinned prompt in full

In the single-pane terminal view, when a Codex pane's capture starts with Codex's pinned prompt
row — "› ", the prompt with its newlines joined, cut at the pane's width and ended with "…",
ending within two cells of the pane's `cols` — and a later row begins with "› " (the
composer), and exactly one of the conversation's ten most recent logged prompts is longer than
the row's text and begins with it (ignoring whitespace), and the row below does not carry on
with that same prompt, the browser mirror SHALL show that full prompt in a bar above the
terminal and SHALL write the capture to the terminal without the row. The bar SHALL show two
lines at rest, open to the whole prompt on a click, and offer Copy. Cell widths SHALL be the
phone's tmux-measured widths. In any other case — another agent, no `cols`, a row short of the
edge, no match, two matching prompts, no composer row, chat mode, a workbench tile — the capture
SHALL be written exactly as received and no bar shown. While the row is on screen and no prompt
explains it, the view SHALL fetch the conversation log at most once every 4 seconds, and stop
once it is explained.

#### Scenario: A cut Codex prompt in the browser

- **WHEN** a Codex pane's top row is cut at its right edge and the latest logged prompt begins
  with it
- **THEN** the bar shows the whole prompt and the terminal starts at the row below it

#### Scenario: Another agent's identical screen

- **WHEN** a Claude Code pane shows the same bytes
- **THEN** the terminal shows them unchanged and no bar appears

#### Scenario: The prompt is logged late

- **WHEN** Codex moves to a new prompt that its log does not hold yet
- **THEN** the cut row shows as captured, and the bar returns within one fetch of the prompt
  being logged

#### Scenario: The same rules as the phone

- **WHEN** the shared case fixture runs against the browser's matcher and the phone's
- **THEN** both give the same answer for every case

### Requirement: Browser entry preserves its Mac path prefix

The browser mirror SHALL load its scripts, styles, fonts and API calls inside the Mac path prefix even when a share URL has no trailing slash. Its asset base SHALL be set before any external asset reference, remain on the current origin, and exclude query strings and credential fragments. Root and explicit index.html entries SHALL use the same API prefix as the document base.

#### Scenario: A guest opens an existing Direct share URL

- **WHEN** a browser opens `/p35047#code=<valid-share-code>`
- **THEN** the page loads its assets from `/p35047/`, removes the fragment before redemption, and redeems through `/p35047/api/enroll`
- **AND** existing scope checks still determine which sessions and inputs the guest can access

#### Scenario: Root or explicit index entry

- **WHEN** the page opens at `/`, `/index.html`, `/p35047/` or `/p35047/index.html`
- **THEN** its asset URLs and API URLs share the correct root or `/p35047` prefix
- **AND** no credential fragment is added to any asset request
