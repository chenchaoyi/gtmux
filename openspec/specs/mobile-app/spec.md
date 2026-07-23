# Mobile App Specification

## Purpose

A phone app (the third surface) to monitor your tmux coding agents remotely, get
lock-screen push when one needs you, and — gated only by the pairing bearer token —
type back into a pane to unstick or steer an agent. Its look mirrors the menu-bar
app, so all three surfaces read as one product.
## Requirements
### Requirement: Pair with a Mac

The system SHALL let the user pair a Mac by host+token, a scanned pairing QR, or a
guest share link, validating reachability + token before saving the pair to the device
Keychain. On receiving a credential the app SHALL detect its KIND: an **enroll code** is
redeemed via `POST /api/enroll` into a `device` (owner, full) token — carried either
by the structured pairing QR or by a pair link (`…/#c=<code>`, the browser medium of
`gtmux pair`), so scanning any pairing medium works; a **guest token**
(the `#g=<token>` carried by a `gtmux share` link/QR; legacy `#t=` links are still accepted) is used directly as the bearer,
without enrollment. After connecting, the app SHALL read `GET /api/share` to resolve its
scope — `all:true` ⇒ owner (full); otherwise a **guest** scoped to the returned
`view_panes` (viewable) and `panes` (typable) — and enter the matching mode.

#### Scenario: Manual pairing

- **WHEN** the user enters the Mac's reachable host and token and connects
- **THEN** the app verifies `/api/health` + an authed call, saves the pair to the
  Keychain, and shows the Radar; a failure gives a plain reachability diagnosis

#### Scenario: Pair as a guest from a share link

- **WHEN** the user opens or scans a `gtmux share` guest link/QR (a `#g=<token>` URL, or a legacy `#t=` one)
- **THEN** the app stores that guest token as its bearer WITHOUT enrolling, reads
  `GET /api/share`, sees `all:false`, and enters guest mode scoped to the returned
  `view_panes`/`panes`

### Requirement: Mirror the status language

The system SHALL render agents with the status language identical to the menu-bar
app — authoritative status colors, the same shapes+glyphs (waiting red square+
pause, working cyan static ring, idle green check, running gray dot), and the
fixed section order waiting→working→idle→running.

#### Scenario: Radar matches the menu bar

- **WHEN** the Radar shows agents
- **THEN** their colors, shapes, glyphs, sectioning, and `primary`/`secondary`
  row text match the menu-bar app

### Requirement: Live radar via SSE

The system SHALL load agents from `/api/agents` and refetch on the `agents` SSE
event, refetch immediately when the app returns to the FOREGROUND (iOS suspends the
SSE stream while backgrounded, so the cached list would otherwise be stale until a
manual pull-to-refresh), show an in-app banner on a foreground `alert`, and reflect
connection state. `/api/agents` is the only data source.

#### Scenario: Live update

- **WHEN** an agent's status changes on the Mac
- **THEN** the Radar updates via the SSE-triggered refetch

#### Scenario: Refresh on foreground

- **WHEN** the app returns to the foreground after being backgrounded
- **THEN** it immediately refetches `/api/agents` so the list is current, independent
  of the (suspended) SSE stream

### Requirement: Detail with terminal + chat views

The system SHALL show a selected agent's Detail in two switchable views kept fresh:
a "终端/terminal" view rendering the pane's live screen via the native pane renderer
(see `mobile-pane-renderer`), and a "对话/chat" view rendering the parsed transcript
(see `mobile-chat-view`, fed by `/api/transcript`). (A phone-side "focus on Mac"
action was removed in #85 — it has little value when you are remote; the `/api/focus`
endpoint stays for the browser mirror + as a stable contract.)

#### Scenario: Terminal view

- **WHEN** the user opens an agent's Detail terminal view
- **THEN** the pane's live screen is rendered (colors, cursor, long-press copy) and
  kept fresh

#### Scenario: Chat view

- **WHEN** the user switches Detail to the chat view
- **THEN** the parsed transcript is shown as a conversation and kept fresh

### Requirement: Push registration + tap deep-link

The system SHALL, when paired and push is enabled, request notification
permission, register the APNs device token to the Mac, and deep-link a tapped
notification to that agent's Detail (including cold start).

#### Scenario: Tap a push

- **WHEN** the user taps a delivered push carrying a `pane`
- **THEN** the app opens to that agent's Detail

### Requirement: Terminal input, gated by the pairing token

The system SHALL let the user type into a pane — literal text (optionally + Enter),
named control keys, and the waiting-pane `1/2/3` approval choices — via
`POST /api/send`, gated ONLY by the pairing bearer token (no separate
authorization), so the token must be treated as a password. After a send, the app
SHALL refresh the pane promptly (not wait for the next poll) so the user sees the
effect of their input quickly; it MAY optimistically echo a sent prompt.

#### Scenario: Send text

- **WHEN** the user types a message in the composer and sends it
- **THEN** the text is delivered via `/api/send` and the pane refreshes promptly to
  show the result

#### Scenario: Answer an approval

- **WHEN** a pane is waiting on a numbered prompt and the user taps a choice
- **THEN** the bare digit is sent via `/api/send` **without a trailing Enter** (the
  agent's numbered menus commit on the digit alone; a trailing Enter would leak onto
  the next prompt and auto-confirm it on consecutive selections) and the pane
  refreshes promptly
- **AND** the choices are presented as a compact row of number chips (`1..N`), not
  re-sketched label rows — the labels are already visible in the terminal/chat

### Requirement: Mobile shows native sessions in an "Elsewhere" section
The mobile app SHALL group `source: "native"` sessions into their own "Elsewhere / 不在 tmux" section, separate from the tmux status groups. These rows are sense-only: they carry a `native` tag, no jump chevron, and tapping one SHALL NOT open a terminal mirror (there is none). Moving a native session into tmux stays a menu-bar/CLI action; the mobile app is display-only for the native category.

#### Scenario: Native section on mobile
- **WHEN** the phone polls the radar and native sessions are present
- **THEN** they SHALL appear in a dedicated "Elsewhere" section, marked non-tappable (no terminal), distinct from the tmux groups

#### Scenario: Tapping a native row does nothing
- **WHEN** the user taps a native row on mobile
- **THEN** the app SHALL NOT navigate to a terminal/detail view for it

### Requirement: Mark errored idle rows in the mobile radar

The mobile radar SHALL visually distinguish an idle agent that ended on an error
(`error: true` in the `agents --json` contract) from a successfully-finished idle
agent, using an amber ⚠ "errored" modifier and the `error_text` summary in place of
the green ✓. The row SHALL remain in the idle section and MUST NOT use the red
`waiting`/needs-you color.

#### Scenario: Errored idle agent

- **WHEN** an agent row has `status: idle` and `error: true`
- **THEN** the mobile radar renders it in the idle section with an amber ⚠ marker
  (not the green ✓) and surfaces the `error_text` summary
- **AND** it is not colored red and does not sort into the needs-you section

#### Scenario: Successful idle agent unchanged

- **WHEN** an agent row has `status: idle` without `error`
- **THEN** the mobile radar renders it exactly as today (green ✓)

### Requirement: The supervisor renders as its own layer (HQ card)

The radar SHALL render a supervisor session (`role:"supervisor"`) as a compact
card below the server chip — NEVER as a row inside the status sections (the
section grouping SHALL exclude supervisor rows). Tapping the card opens the
supervisor's Detail in CHAT mode (conversing with the supervisor is the primary
mobile path). When no supervisor is live the card is simply absent (starting one
requires the Mac; the phone shows no dead control).

#### Scenario: Supervisor live on mobile

- **WHEN** `/api/agents` includes a `role:"supervisor"` row
- **THEN** the radar shows the HQ card below the server chip, the row is excluded
  from the sections, and tapping the card opens its Detail in chat mode

#### Scenario: Supervisor absent on mobile

- **WHEN** no row carries `role:"supervisor"`
- **THEN** no HQ card (and no dead "start" control) is shown

### Requirement: The supervisor opens a HQ command center, not the generic detail

When the user opens a `role:"supervisor"` session on mobile, the app SHALL present a
dedicated HQ command center — NOT the generic Chat/Terminal detail — and that command
center SHALL be built from what only the supervisor knows, NOT from a second rendering of
the radar. It SHALL NOT list the fleet session-by-session: the per-session list belongs to
the radar, and repeating it here adds no information (fleet COUNTS remain in the status
strip). It SHALL contain, in order: a status strip (fleet counts + subscription-window %
+ resource warning); an ASSESSMENT zone (a deterministic one-line conclusion about what
needs the user, plus access to the supervisor's own situation board with its freshness);
and three switchable zones, each given the full body height rather than a share of it: a
YOUR-CALL zone (one decision card per waiting session, each showing that session's ask as
the card's body rather than as a footnote, and offering both opening that session directly
and asking the supervisor to draft a reply), an ACTIVITY zone (the event ledger at notable
severity and above), and a CONSOLE zone (a conversation with the supervisor). The command
bar — free text plus quick-command chips — SHALL remain available on every zone, since the
user can always have something to say to the supervisor. The zone selector SHALL carry
each zone's own signal (how many decisions are pending, whether activity is new) so the
zones the user is NOT looking at still report themselves. The app SHALL open on the
your-call zone when something is waiting and on the console otherwise, because the reason
to open HQ while a session is blocked is the block. Commands are HQ-mediated: the command
bar addresses the supervisor, which drives the fleet; the HQ screen has NO direct-send
input of its own — direct control lives in each worker's own detail. Every zone SHALL
state its empty condition in words; NO zone may render as a bare header over blank
space.

#### Scenario: Open the supervisor

- **WHEN** the user taps the gtmux HQ card (a `role:"supervisor"` row)
- **THEN** the HQ command center opens with the assessment, your-call, activity and
  console zones, not the generic Chat/Terminal segmented detail

#### Scenario: The fleet is not listed twice

- **WHEN** the user is in the HQ command center with several sessions running
- **THEN** no per-session fleet list is shown, and the sessions are represented only by
  the counts in the status strip and by decision cards for those actually waiting

#### Scenario: A waiting session's ask is the decision

- **WHEN** a session is waiting on the user
- **THEN** a decision card names it and shows its ask as the card's body, and offers
  opening that session directly as well as asking the supervisor to draft the reply

#### Scenario: Nothing needs the user

- **WHEN** no session is waiting
- **THEN** the your-call zone says so plainly instead of rendering empty

#### Scenario: A zone reports itself while hidden

- **WHEN** two sessions are waiting and the user is on the console zone
- **THEN** the your-call zone's selector still shows that two decisions are pending

#### Scenario: Opening HQ while blocked

- **WHEN** the user opens HQ and at least one session is waiting
- **THEN** the your-call zone is the one shown first

#### Scenario: Selecting a decision card targets a command

- **WHEN** the user selects a decision card
- **THEN** per-target quick actions (e.g. continue / inspect / reply-for-me) become
  available in the command bar, addressed to that session through the supervisor

### Requirement: The supervisor's own assessment is readable from the app

The app SHALL make the supervisor's situation board readable on the phone, with the time
it was last updated, so the user can see the supervisor's synthesis without opening its
terminal. It SHALL be presented read-only — the board is the supervisor's own working
memory, and the app is not an editor for it. When the supervisor keeps no board, or its
data is unavailable, the app SHALL degrade to the deterministic assessment line rather
than showing an error or an empty panel.

#### Scenario: Read the board

- **WHEN** the user opens the situation board from the HQ command center
- **THEN** the board's content is shown read-only together with how long ago it was
  last updated

#### Scenario: No board yet

- **WHEN** the supervisor has written no situation board
- **THEN** the assessment zone shows the deterministic conclusion alone, with no error
  and no empty panel


### Requirement: Guest mode is scoped and hides owner-only surfaces

When paired as a guest (`GET /api/share` returns `all:false`), the app SHALL confine
itself to the guest scope and SHALL NOT expose owner-only surfaces. It SHALL show only
the sessions on the view allowlist (the guest-filtered `/api/agents`), offer an input
affordance only on panes in the input allowlist, and HIDE the owner-only surfaces:
usage, the digest/HQ command console, the device roster/management, the share controls,
and the Anywhere/tunnel/remote-access configuration. It SHALL fail safe — never call an
owner-only endpoint (which `403`s), degrading rather than erroring — and SHALL show a
persistent banner naming the host and the count of scoped sessions, so the restricted
scope is never ambiguous.

#### Scenario: Guest sees only allowed sessions

- **WHEN** a guest-paired app loads the Radar
- **THEN** it shows only the host's view-allowed sessions, with an input affordance only
  on input-allowed panes, and a non-viewable pane's screen is never shown (`/api/pane`
  `403`)

#### Scenario: Owner-only surfaces are hidden for a guest

- **WHEN** a guest-paired app renders its UI
- **THEN** usage, the digest/HQ console, device management, the share controls, and
  remote-access config are not shown, and their owner-only endpoints are never called

#### Scenario: A revoked guest link ends access

- **WHEN** the host runs `gtmux share revoke <id>` for that guest's link
- **THEN** the guest app's calls stop being authorized and the app returns to its
  pairing screen rather than showing stale data

### Requirement: The app separates paired Macs from guest connections

The app's server list SHALL present the two-track model: paired Macs (owner
scope) under "我的 Mac/My Macs" and share-link connections (guest scope) under
"访客连接/Guest access", never intermixed. A guest connection SHALL display its
granted scope (how many sessions are viewable and how many typable, from
`GET /api/share`), and guest-mode copy SHALL say it is connected via a share
link (分享) rather than paired (配对).

#### Scenario: The list reads the two tracks

- **WHEN** the user has one paired Mac and one share-link connection saved
- **THEN** the server list shows the Mac under 我的 Mac and the guest connection
  under 访客连接, the latter labelled with its granted scope

#### Scenario: A guest connection shows its access

- **WHEN** the app is connected over a share link that grants 2 viewable / 1
  typable sessions
- **THEN** the guest banner/scope line reads that count, sourced from the
  caller-scope endpoint

### Requirement: An owner-only screen manages this Mac's sharing

The app SHALL offer a "Manage this Mac" screen, reachable ONLY on an owner
connection (a paired device — `!isGuest`); a guest connection SHALL NOT surface
its entry. The screen SHALL let the owner manage SHARING for the connected Mac,
mirroring the menu bar: toggle the consent switch, see each share link with its
per-link scope, edit a link's See/Type per session, create a new link (name +
per-session scope in one step), and revoke a link. It SHALL also show the paired
DEVICE roster READ-ONLY, with a one-line note that revoking a device and changing
the remote-access door are done on the Mac (decision B). The screen SHALL NOT
present controls for the withheld actions, so no button 403s.

#### Scenario: An owner opens the management screen

- **WHEN** the app is connected with a device (owner) token
- **THEN** the "Manage this Mac" entry is available, and it shows the share
  controls (consent, per-link See/Type, create, revoke a link) plus a read-only
  device roster

#### Scenario: A guest never sees management

- **WHEN** the app is connected via a share link (guest)
- **THEN** the "Manage this Mac" entry is absent, and the app never calls the
  management endpoints

#### Scenario: The owner edits a link's scope from the phone

- **WHEN** the owner toggles a session's Type on a link and confirms
- **THEN** the app calls `POST /api/share/set` for that link only, and the change
  is reflected (per-link, not global)

### Requirement: Mobile HQ card shows an intelligence headline, not fleet pips

The mobile HQ (chief-of-staff) card SHALL NOT render a row of per-worker "fleet pips"
(they duplicate the section list below it). Its subtitle SHALL be the same deterministic
intelligence headline as the menu-bar card: it names the worker that needs the user plus
a count of the rest when something is waiting, or reads as "all normal" when quiet,
coloured for attention when a worker or HQ itself needs the user.

#### Scenario: A worker is waiting

- **WHEN** the fleet has one or more waiting workers
- **THEN** the mobile HQ card subtitle names the first waiter and how many others are normal, with attention colour, and renders no pip row

#### Scenario: All quiet

- **WHEN** no worker is waiting
- **THEN** the mobile HQ card subtitle reads as "all normal", dim, with no pip row

### Requirement: Only an actual menu is offered as an approval card

The system SHALL present numbered choices only when the agent is actually offering a menu,
and SHALL distinguish a menu from a numbered LIST in ordinary output. A menu marks its
highlighted row with a selector; prose never does. Being blocked on the user is not
sufficient evidence, because an agent can be waiting on a free-form question while its
recent output happens to contain a numbered list — presenting that list as choices offers
options the agent never made, on a control that invites a single keypress to answer with.
Where no menu can be identified the system SHALL present no card, leaving the user to
reply in their own words.

#### Scenario: A numbered list in prose

- **WHEN** a waiting session's output contains a numbered list that is not a menu
- **THEN** no approval card is shown

#### Scenario: A real menu after prose

- **WHEN** the output contains both a numbered list and a genuine menu
- **THEN** the card offers the menu's choices

### Requirement: A full-screen reader is escapable and clear of system UI

A full-screen reader the app presents SHALL be laid out clear of the device's own status
bar, and SHALL offer an unmistakable way out. Presenting it as a plain modal is not
sufficient: a modal is rendered in its own hierarchy where safe-area insets resolve to
zero, so its header and its close control are drawn underneath the clock and battery —
illegible, and overlapping system UI that intercepts the touch. The way out SHALL be
labelled rather than a bare glyph, and SHALL NOT be the only one, so leaving never depends
on hitting a single small target. Content authored as markup SHALL be rendered, not shown
as its source.

#### Scenario: Opening the reader

- **WHEN** a full-screen reader is presented
- **THEN** its header and close control sit below the system status bar

#### Scenario: Leaving

- **WHEN** the user wants to leave the reader
- **THEN** a labelled control and a gesture both dismiss it

#### Scenario: Markup content

- **WHEN** the content is markdown
- **THEN** it is rendered as formatted text, not as raw markup
