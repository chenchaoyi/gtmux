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

#### Scenario: A pairing request nothing answers ends with a diagnosis

- **WHEN** a pairing step (the enroll redeem, the health check, or the authed check) gets
  no answer within 15 seconds, for example because a VPN on the phone swallows it
- **THEN** the app stops waiting (the enroll request is cancelled), leaves the spinner,
  and reports the server as unreachable, with a hint to retry with any VPN or proxy off;
  "token rejected" is shown only when the server answered 401/403, never for a request
  that timed out, dropped, or got an edge's 5xx

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

### Requirement: An unreachable Mac leaves its last state on screen, muted

When the Mac cannot be reached the app SHALL keep what it last knew rather than clearing
the screen, and SHALL mute it: the rows and their section bars drop to half strength while
the banner above them, which carries the reason and the Retry, stays at full. The header
stays at full too, since its buttons still work. The ages in those rows go on counting
while the Mac is away, so a list at full strength reads as current when none of it is.

#### Scenario: The Mac stops answering

- **WHEN** the phone cannot reach the Mac
- **THEN** the agent list stays on screen at half strength under a full-strength banner,
  and returns to full strength when the Mac answers again

### Requirement: The live connection rebuilds itself after the Mac goes away

The app SHALL recover the live stream on its own after the Mac stops answering
entirely — a serve restarted by `gtmux update`, a Mac that slept, a tunnel that
blinked — and SHALL NOT require a relaunch, a foreground cycle or a tap on Retry. It
SHALL rebuild the subscription on a stream error with a backoff no longer than 30
seconds, read `/api/agents` over HTTP on each attempt so the board is current even
while the stream is still refused, and refetch once when the stream comes back, since
changes made while it was gone were never sent. An outage SHALL be recorded once in
the phone's diagnostic record, not once per attempt.

#### Scenario: The Mac's serve restarts

- **WHEN** the Mac's serve stops for half a minute and comes back at the same address
- **THEN** the app returns to connected on its own, and a fleet change made after it
  came back reaches the radar without a pull-to-refresh

#### Scenario: A long outage

- **WHEN** the Mac is unreachable for several minutes, across several retries
- **THEN** the diagnostic record holds one "live connection dropped" entry for that
  outage, and one entry saying how long it was gone when it returns

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
the radar, and repeating it here adds no information.

**The standing header SHALL carry the supervisor's judgment and nothing else** — the
page's identity, its connection state, and the one-line verdict. Everything else the
header once carried standing (fleet counts, subscription window, resource line, the board
entry) SHALL move behind a disclosure on that verdict, because the interactive zone below
pays for every standing pixel and a keyboard leaves it only a few lines. The disclosure
SHALL open with the supervisor's OWN most recent brief where it has one, its words rather
than recomputed counts, since the supervisor already produces a periodic brief and a
tally of states is the weaker answer to "what is going on". A resource condition SHALL be
promoted OUT of the disclosure only at the critical tier, which the verdict already
models.

It SHALL contain three switchable zones, each given the full body height rather than a
share of it: a YOUR-CALL zone (one decision card per waiting session, each showing that
session's ask as the card's body rather than as a footnote, and offering both opening that
session directly and asking the supervisor to draft a reply), a zone for the SUPERVISOR'S
OWN ACTS with the fleet ledger available beside it, and a CONSOLE zone (a conversation
with the supervisor). The command bar — free text plus quick-command chips — SHALL remain
available on every zone. The zone selector SHALL carry each zone's own signal so the zones
the user is NOT looking at still report themselves. The app SHALL open on the your-call
zone when something is waiting and on the console otherwise. Commands are HQ-mediated: the
command bar addresses the supervisor, which drives the fleet; the HQ screen has NO
direct-send input of its own. Every zone SHALL state its empty condition in words; NO zone
may render as a bare header over blank space.

#### Scenario: Open the supervisor

- **WHEN** the user taps the gtmux HQ card (a `role:"supervisor"` row)
- **THEN** the HQ command center opens with the verdict, your-call, acts and console
  zones, not the generic Chat/Terminal segmented detail

#### Scenario: The standing header does not crowd the conversation

- **WHEN** the user is typing to the supervisor
- **THEN** the standing header is the identity row and the verdict line only, with the
  fleet counts, resource line and board entry reachable through the verdict's disclosure

#### Scenario: The supervisor's own brief leads the disclosure

- **WHEN** the supervisor has produced a brief and the user opens the disclosure
- **THEN** the supervisor's own most recent brief is what it opens with, and the derived
  counts follow it

#### Scenario: A machine at its critical tier

- **WHEN** the machine reaches its critical resource tier
- **THEN** the condition is stated in the standing header rather than only inside the
  disclosure

#### Scenario: The fleet is not listed twice

- **WHEN** the user is in the HQ command center with several sessions running
- **THEN** no per-session fleet list is shown, and the sessions are represented only by
  the counts in the disclosure and by decision cards for those actually waiting

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
and SHALL distinguish a menu from a numbered LIST in ordinary output. Being blocked on the
user is not sufficient evidence, because an agent can be waiting on a free-form question
while its recent output happens to contain a numbered list — presenting that list as
choices offers options the agent never made, on a control that invites a single keypress
to answer with.

TWO independent conditions SHALL hold, because each has failed on its own:

- **The agent SHALL have asked.** The evidence is the waiting KIND recorded from the
  agent's own hook event (a permission request, a plan, or a question) — NOT the mere
  presence of a waiting state. gtmux also marks a pane waiting on its OWN screen
  inference (a dispatch it believes is stuck before running), and such a pane's agent has
  asked nothing; that state SHALL NOT qualify. A waiting state of unknown provenance SHALL
  read as no ask.
- **The screen SHALL show a live menu.** A menu marks its highlighted row with a selector
  cursor and the cursor LEADS that row; prose never does. A selector glyph appearing
  ELSEWHERE on a numbered line SHALL NOT qualify — several of those glyphs (`→`, `>`) are
  ordinary characters in prose and code.

An agent that emits no hook events at all offers no card, and the user replies in the
terminal — a degradation, never a hard failure.
The system SHALL further present a card ONLY for a CLEAN, SINGLE-select, tap-to-reply menu
a bare number-send can drive. It SHALL NOT present a card for a RICH picker the one-tap
card cannot express: a side-by-side preview picker (Claude Code's `AskUserQuestion`
renders each option beside a preview panel on the same line, so the parsed label swallows
the preview via an interior box rule, and it navigates by arrows/enter rather than a bare
number), OR a MULTI-select picker (each option marked with a `[ ]` / `[x]` checkbox — the
card can only single-tap-and-submit, so it cannot express "check 1 and 3, then submit").
Where no replyable menu can be identified the system SHALL present no card, leaving the
user to reply in their own words / in the terminal.

#### Scenario: A numbered list in prose

- **WHEN** a waiting session's output contains a numbered list that is not a menu
- **THEN** no approval card is shown

#### Scenario: A prose bullet containing an arrow is not a menu row

- **WHEN** a numbered line in ordinary output contains a selector glyph somewhere inside
  it (e.g. a findings bullet reading `(config.toml enabled = false)→ gtmux 收不到`) rather
  than as a cursor leading the row
- **THEN** the run is still read as a LIST and no approval card is shown

#### Scenario: A pane gtmux itself inferred was stuck offers no card

- **WHEN** a pane is marked waiting by gtmux's own screen inference (a dispatch believed
  stuck before running) and its agent has asked nothing
- **THEN** its choices are not parsed at all and no approval card is shown, however
  menu-like the screen text looks

#### Scenario: A real menu after prose

- **WHEN** the output contains both a numbered list and a genuine menu
- **THEN** the card offers the menu's choices

#### Scenario: A rich preview picker is not offered as a card

- **WHEN** a waiting session shows a rich picker (options rendered beside preview panels
  on the same line, driven by arrows/enter — e.g. `AskUserQuestion`)
- **THEN** no approval card is shown (its parsed labels are contaminated and a bare
  number-send cannot drive it), and the user replies in the terminal instead

#### Scenario: A multi-select picker is not offered as a one-tap card

- **WHEN** a waiting session shows a multi-select picker (options marked with `[ ]` / `[x]`
  checkboxes, toggled then submitted)
- **THEN** no approval card is shown (a one-tap-and-submit card cannot express a
  multi-selection), and the user replies in the terminal instead

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

### Requirement: Demo mode never shows or writes real user data

In demo mode the app SHALL present only sample data and SHALL NOT read from, or write to,
any store that holds the user's real content. The composer's input history is such a
store: it holds the actual messages the user typed against their own machine, and showing
those inside the demo both exposes them and breaks the demo's guarantee of being a
self-contained sample. In demo, the input history SHALL be a canned sample list, and
messages typed in demo SHALL NOT be persisted to the real history — so the demo neither
reveals real history nor grows it.

#### Scenario: Opening history in demo

- **WHEN** the user opens the composer's input history in demo mode
- **THEN** it shows a canned sample list, not messages typed against a real machine

#### Scenario: Typing in demo

- **WHEN** the user sends a message in demo mode
- **THEN** it is not added to the real input history

### Requirement: The collapsing top chrome never resizes the scroll view under it

The detail screen and the HQ page fold their top chrome away while the reader is in
history and bring it back at the live tail. The chrome SHALL float over the scroll
view and fold by sliding, so the scroll view's frame does not change; the content
carries a constant top padding of the chrome's height. Folding by animating the
chrome's height changes the very distance the fold decision reads, and the header
argues with itself (measured on both screens: 2026-09-05 detail, 2026-09-12 HQ).

A zone whose content reads downward from under the chrome (the HQ page's "your call"
and "HQ's work") SHALL NOT fold at a threshold: a fold there leaves a blank band above
the first row. Its chrome scrolls away with the content, in step, clamped at the
chrome's own height, and returns the same way.

The tail-anchored views (the conversation, the terminal) have the same band at the top
of their content, and the chrome SHALL come back over it: the distance the fold rule
reads is the smaller of the distance from the tail and the distance past the band's end,
so a reader at the top of the content sees the chrome, not a blank strip its height. A
conversation short enough that both edges are within the fold line never folds.

#### Scenario: A small scroll into history on the HQ console

- **WHEN** the reader scrolls the HQ console into history by a little more than the
  fold line but less than the chrome's height
- **THEN** the chrome folds once and stays folded; it does not flicker, and the reader
  is not pulled back to the tail

#### Scenario: The top of a short conversation

- **WHEN** the reader scrolls the HQ console up to "Load the earlier session" after a
  `/clear` left the session a few turns long
- **THEN** the chrome is shown over its padding band and the session-start line sits
  directly under it; the band is never seen blank (2026-09-16: half a screen of nothing
  above the session-start line, with the chrome folded and nothing to unfold it)

#### Scenario: A small scroll in a top-anchored zone

- **WHEN** the reader scrolls "HQ's work" by less than the chrome's height
- **THEN** the chrome has moved up by exactly that much and no blank band shows above
  the first row; scrolling back to the top brings it fully back

### Requirement: Demo mode shows the same product the radar does

Demo mode is the only view of the app App Review gets, and a new user's first
impression of it. It SHALL therefore offer the same surfaces the real radar offers,
over sample data — not a reduced version of them. A surface reachable from the real
radar and absent from the demo is a feature a reviewer cannot see and a user is not
shown.

Demo-only affordances (a sample-data banner, a "pair your Mac" call to action, a
close control) are expected and SHALL remain. What must not differ is the product:
the fleet tally, the filters, the sections, the floating supervisor control, the
detail view, and the all-panes browser.

Where a piece of chrome is drawn on both, it SHOULD be one shared component rather
than a copy in each — the demo fell behind the real radar twice by being a copy.

#### Scenario: A surface exists on the radar but not in demo

- **WHEN** the real radar offers a surface (for example the all-panes browser)
- **THEN** the demo offers it too, over sample data, reachable the same way

#### Scenario: A pane opened from the demo browser

- **WHEN** a pane is opened from the demo's all-panes browser, including a plain
  (non-agent) pane
- **THEN** it shows a believable sample screen rather than an empty one

### Requirement: Native terminal text selection on iOS

The iOS terminal viewer SHALL provide system-native text selection over the
colored terminal rendering: long-press selects the word under the finger with
the standard selection band and BIDIRECTIONAL drag handles, and Copy places
exactly the selected range on the clipboard. Selection geometry SHALL be
supplied by the app (uniform row grid + measured character advances), never
inferred by a second text-layout pass, so the band always lands on the glyphs
the user sees regardless of CJK content or line wrapping.

#### Scenario: Select a range anywhere in the buffer

- **WHEN** the user long-presses a line — at the live tail or deep in
  scrollback — and drags either handle in either direction
- **THEN** the selection band tracks the touched glyphs exactly, the terminal
  does not jump or auto-scroll, and Copy yields precisely the banded text.

#### Scenario: Any Dynamic Type size

- **WHEN** the device's text size (Dynamic Type) is set to any value — below,
  at, or above the default — and the user long-presses anywhere in the buffer,
  including the bottom-most rows at the live tail
- **THEN** selection works identically across the whole buffer: the rendered
  terminal honors the OS text size, and every geometry consumer (row wrap,
  row height, the selection overlay's font) derives from the same effective
  font size, so no region of the buffer is dead to selection.

#### Scenario: Zero standing cost

- **WHEN** no selection is active
- **THEN** the selection layer performs no text layout and typing/scrolling
  performance is unaffected by its presence.

### Requirement: What's new is shown once after an update, in the reader's language

After the app updates, it SHALL show the release notes for EVERY version the reader crossed
— not only the newest — ONCE, and SHALL make them reachable again from Settings, because a
changelog that can be seen only once cannot be gone back to. This is the phone's
counterpart to `gtmux whatsnew`, and it SHALL follow the same TWO-LAYER shape: the popup is
a CAPPED summary (someone who just updated is on their way somewhere else), with the full
list one tap away.

Because a user may skip several releases, the notes SHALL be a PER-VERSION ARCHIVE carried
in the binary. The App Store metadata cannot serve: it holds only the current submission's
text. The archive SHALL be generated, not authored a second time, and a conformance check
SHALL fail when the generated form and the archive disagree — the generation is a release
step someone will eventually forget.

Versions SHALL be ordered by numeric segment (so 0.10 follows 0.9), newest first, and
entries newer than the RUNNING build SHALL be excluded — a checkout's archive can be ahead
of the binary, and promising a user something their build does not have is worse than
saying less.

When the summary is capped:

- a version SHALL be shown WHOLE or folded — never with a partial list of its bullets,
  since "3 of 6 changes in 0.46.0" is a claim no reader can act on;
- the fold SHALL be a SUFFIX, never a gap: once a version does not fit, everything older
  folds with it, because skipping a version to fit a smaller one behind it would tell the
  reader that the skipped release changed nothing;
- the NEWEST version SHALL always be shown, even when it alone exceeds the cap;
- the remainder SHALL be named with its count and expandable in place.

A version's notes SHALL be laid out as written: a line beginning `- ` is an item under
the heading line before it, and a heading carries no bullet of its own. A release written
without `- ` lines stays a flat list of bullets.

The card SHALL carry the product's own identity rather than generic chrome — the pane-grid
brand mark that is also the app icon, and versions set in the same monospace the terminal
uses — and nothing beyond it: no section taxonomy, no accent fills, no animation. A version
SHALL be named ONCE, so the header's version and the per-version headings never both print
it.

Nothing SHALL wrap the scrolling region in a pressable: a press responder taken on touch
START prevents the scroll view from ever claiming the gesture, which makes the card scroll
in fits or not at all. A dismiss-on-backdrop affordance SHALL therefore be a SIBLING behind
the card, never an ancestor of it.

The bullets SHALL be rendered in the reader's resolved language (the system/EN/中文
setting), falling back to the other language when its own is absent — the same fallback the
CLI makes between a tag's `user:` and `user-zh:` blocks — and the cap SHALL count the
language actually being read.

The popup SHALL NOT appear on a FRESH INSTALL: there is no "new" for someone who has never
seen the old. That install SHALL be recorded silently, so the first UPDATE is what greets
them. A version with nothing archived SHALL likewise show nothing and record.

#### Scenario: A multi-version jump reports every version crossed

- **WHEN** the reader last acknowledged 0.45.0 and now runs 0.47.0, with notes archived for
  0.46.0 and 0.47.0
- **THEN** both versions' notes are shown, newest first, grouped under their versions

#### Scenario: An ordinary update is not truncated

- **WHEN** a single version's notes fit within the summary cap
- **THEN** they are all shown and no fold appears

#### Scenario: A long history folds into a counted remainder

- **WHEN** the crossed versions carry more bullets than the cap
- **THEN** the newest versions are shown whole, the rest are folded as a counted remainder,
  and expanding it reveals them in place

#### Scenario: A fresh install is not greeted

- **WHEN** the app launches for the first time
- **THEN** nothing is shown and the running version is recorded silently

#### Scenario: The notes follow the language setting

- **WHEN** the reader's resolved language is Chinese
- **THEN** the Chinese bullets are shown, falling back to the English ones only if the
  Chinese notes are absent

#### Scenario: The card scrolls

- **WHEN** the notes are longer than the card
- **THEN** they scroll normally — no pressable is an ancestor of the scrolling region, so
  the gesture is never stolen on touch start

#### Scenario: A version is named once

- **WHEN** several versions are reported and each carries a heading
- **THEN** the header does not repeat the newest version's number

#### Scenario: The notes stay readable after dismissal

- **WHEN** the user opens Settings → About
- **THEN** the full archive can be opened again on demand

### Requirement: The layout adapts to the canvas, in either orientation

The two-column (sidebar + detail) layout SHALL be chosen by the window's WIDTH **and**
HEIGHT, not width alone. A modern phone in landscape is wider than the width breakpoint and
barely 400pt tall; giving it the tablet layout is the inverse of the rule that a tablet is
not a big phone. The height threshold SHALL clear every tablet orientation and no phone.

In FULL-SCREEN reading mode, the floating exit control SHALL NOT cover content: the content
SHALL begin below it. This costs a little of the reading space full-screen exists to
maximize, and that is the correct trade — space the reader can see beats space that hides a
line, which is most acute in landscape where the whole reading area is a few hundred points
tall.

Safe-area insets SHALL be applied HORIZONTALLY as well as at the top, because in landscape
the notch/Dynamic Island sits on a side; with only the top edge applied, the terminal and
the transcript both render underneath it. (Horizontal insets are zero in portrait, so this
affects landscape alone.)

#### Scenario: A phone in landscape keeps the single-column layout

- **WHEN** the window is wider than the split breakpoint but only a few hundred points tall
  (a phone rotated to landscape)
- **THEN** the stacked phone layout is used, not the tablet's sidebar-plus-detail

#### Scenario: A tablet gets the split layout in both orientations

- **WHEN** the window is a tablet canvas in portrait or landscape
- **THEN** the two-column layout is used

#### Scenario: The full-screen exit control never hides a line

- **WHEN** the reader enters full-screen in either mode
- **THEN** the content starts below the floating exit control rather than beneath it

#### Scenario: Landscape content clears the notch

- **WHEN** the device is in landscape with the notch on one side
- **THEN** the content is inset horizontally so nothing renders under it

### Requirement: The HQ page headline renders the core's verdict

The HQ page's assessment headline SHALL render the verdict served with the digest rather
than deriving one from the rows it happens to display. Deriving it locally is what let the
phone report a quiet fleet while the same moment's menu bar reported a machine under
pressure — the phone's local rule considered neither the supervisor's own waiting state
nor the machine's resource tier.

The sentence SHALL be composed on the device, in the reader's own language, from the
served state and facts. When no verdict is served the page SHALL fall back to its local
derivation rather than showing nothing.

#### Scenario: The phone speaks on a state it used to miss

- **WHEN** the served verdict is the supervisor's own call, or the machine's resource
  tier, while every worker is idle
- **THEN** the HQ page headline reports it, in the reader's language, instead of "all
  normal — nothing needs you"

### Requirement: Long-pressing a session says what the row cannot

A radar row is clamped for density, so the long-press exists to reveal what the clamp
hides. It SHALL show the full task, the full error text where the session ended on one,
and the background-work note where work is still in flight — and it SHALL identify the
pane it belongs to, leading with the pane id, because a truncated name cannot tell two
panes running one project apart.

It SHALL NOT merely restate the row. Repeating the agent name and the task — both already
visible on the row that was pressed — spends a deliberate gesture on nothing.

It SHALL offer the step that is otherwise two screens away: jumping the Mac's terminal to
that pane, and copying the command that does the same thing from a shell. It SHALL NOT
offer a destructive action; a long-press is a browsing gesture.

#### Scenario: A task or error too long for the row

- **WHEN** a row's task or error text is clamped
- **THEN** the long-press shows it in full

#### Scenario: Identifying which pane this is

- **WHEN** the sheet opens for a pane in tmux
- **THEN** it leads with the pane id, with the session/window location beside it

### Requirement: Each kind of row tells its own truth

The radar carries agent sessions, sessions sensed outside tmux, and plain panes a user
promoted onto the list. They are different things, and the sheet SHALL NOT present them
identically.

A session sensed outside tmux SHALL be shown as such, and the actions that require a pane
SHALL be offered as unavailable WITH THE REASON rather than silently absent or,
worse, offered and broken. A watched plain pane SHALL NOT be given an agent status it
does not have.

#### Scenario: A session that is not in tmux

- **WHEN** the sheet opens for a row sensed outside tmux
- **THEN** it says so, and the jump is unavailable with the reason given

#### Scenario: A watched plain pane

- **WHEN** the sheet opens for a promoted plain pane
- **THEN** it shows the pane and its command, and no agent status

### Requirement: The HQ page shows what the supervisor DID, not only what the fleet did

A supervisor that acts on the user's behalf and never shows its work is indistinguishable
from a dashboard. Measured over one week on a real machine, the supervisor dispatched
work 27 times, reclaimed 4 finished dispatches, recorded 168 knowledge entries, audited
its own products 8 times and rotated its own aged session — and none of it was readable
from the app, whose activity zone showed the FLEET's lifecycle instead.

The HQ page SHALL present the supervisor's own acts — dispatch, reclaim, record, audit,
rotate, and alarms about the supervision itself — as a first-class zone, each act naming what it
acted on, what it was, and how it ended, with a tally over a recent window. The fleet's
lifecycle ledger SHALL remain available beside it as a filter rather than as the default,
because a session starting or stopping is not news and a chief of staff dispatching work
is.

These acts SHALL be read from the journal the core already serves, narrowed by the core
BEFORE its record cap: the acts are sparse in a feed the wake plumbing dominates, and a
client filtering after the cap sees hours where the reader needs a week. Wake DELIVERY is
not an act — it is the core knocking on the supervisor's door, not the supervision
working. An act kind the client does not recognise SHALL still render as a
readable row rather than a raw token, so a newly journalled kind degrades instead of
leaking an identifier at the user.

#### Scenario: The supervisor dispatched work while the user was away

- **WHEN** the supervisor dispatched a task to a worker and the user opens the HQ page
- **THEN** that dispatch appears as an act naming the worker, what was sent and whether
  it landed

#### Scenario: The fleet ledger is still reachable

- **WHEN** the user wants the session lifecycle rather than the supervisor's acts
- **THEN** a filter beside the acts shows the fleet ledger, without either view dropping
  records the other holds

#### Scenario: An unrecognised act kind

- **WHEN** the journal carries an act kind this client predates
- **THEN** the row still renders in words rather than as a raw event token

### Requirement: The HQ console reaches the session before a clear

HQ starts over often (`/clear`, `/new`, `gtmux hq --rotate`), and each start is a new
session log, so the console showed only what came after the last one (2026-09-15:
「每次只能展示上一次 clear 后的一点内容」). The serve SHALL stitch earlier HQ sessions in
front of the current one on request (`?earlier=N`), following the `hq-session` audit
chain, marking the first turn of each later session with a break; the console SHALL
draw that seam (the clock and the command that began the new session) and SHALL offer
「载入上一段对话」 above the oldest turn while the serve reports one more session, asking
for one more hop per tap. A worker's Detail, which has no chain, offers nothing.

#### Scenario: Reading back past a clear

- **WHEN** HQ cleared at 23:03 and the reader taps 「载入上一段对话」
- **THEN** the turns of the session before the clear appear above, and between them and
  the current session a seam reads 「— 新一段对话 · 23:03 /clear —」; the offer stays while
  an even earlier session is known

### Requirement: The HQ conversation shows the work while it happens

A supervisor can take minutes over one turn. Showing only an elapsed timer while it works
and then folding its steps behind a dim toggle once it stops presents the process exactly
when it cannot be watched and hides it while it can.

The HQ console SHALL surface the in-flight turn's steps as they arrive, falling back to
the elapsed-time line only while there is genuinely nothing yet — never inventing an
elapsed time it does not have. The CURRENT turn's steps SHALL be shown expanded and
history SHALL stay collapsed: watching and archaeology are different needs.

#### Scenario: The supervisor is working and has taken steps

- **WHEN** the supervisor is mid-turn and has produced steps
- **THEN** the console shows those steps as they arrive, rather than only an elapsed timer

#### Scenario: The supervisor is working and has produced nothing yet

- **WHEN** the supervisor is mid-turn with no step recorded
- **THEN** the console shows the elapsed-time line, and shows no duration at all when the
  start time is unknown

#### Scenario: The turn finishes

- **WHEN** the turn completes
- **THEN** its steps collapse to the history presentation

### Requirement: A send that did not land says why, in the core's own words

When `POST /api/send` refuses, the app SHALL show the reason the core gave rather than a
generic failure. The three refusals differ in what the reader should do — someone is
typing in that pane, the pane is gone, the key is not allow-listed — and one bar that
cannot tell them apart sends the reader to the Mac for all three.

No override is offered. `POST /api/send` carries no field for one — the core's
draft-clobber switch is reachable only from the CLI — so an "send anyway" button would
either lie or require a contract change, and a contract change is not a display decision.
The copy says what is true and what the reader can do instead.

#### Scenario: A pane with someone typing in it

- **WHEN** a send is refused because the pane holds unsent text
- **THEN** the app says so in the core's words, keeps the typed message, and does not
  offer an override the API cannot carry

#### Scenario: A pane that is gone

- **WHEN** a send is refused because the pane no longer exists
- **THEN** the app says so, and offers no override

### Requirement: A send into a working session says it will wait

A message sent into a session that is mid-turn is not lost — the agent queues it behind
the current turn — but "delivered" and "queued behind a turn that may run for minutes" are
different facts and the reader acts on them differently. The app SHALL say which it was.

It reads this off the target's status, which the radar already carries, rather than adding
a screen read to a path built to answer immediately. That makes it an inference rather
than a measurement, and it is worded as the expectation it is: the status can be briefly
stale after a missed transition, and a sentence about what will happen next survives that
where a claim about what DID happen would not.

#### Scenario: Sending into a session that is working

- **WHEN** a send lands in a pane whose status is working
- **THEN** the app says the message will be handled when the current turn ends

#### Scenario: Sending into an idle session

- **WHEN** a send lands in a pane that is not mid-turn
- **THEN** the app says nothing extra, because there is nothing extra to say

### Requirement: The phone's knowledge sheet shows the axes and carries an entry

The knowledge sheet SHALL show kind, provenance and audience, group the pool by
neighbourhood, and offer the same carry / feedback / withdraw acts through
`POST /api/hq/knowledge/act`, owner-only.

#### Scenario: A guest opens the sheet

- **WHEN** a guest token reads `/api/hq/knowledge`
- **THEN** it is refused, as every `/api/hq/*` surface is

### Requirement: The knowledge sheet follows the app's language for content too

The knowledge sheet SHALL show each entry's half matching the app's language, fall back to
the other half with a small language tag, and match find against both halves.

#### Scenario: A Chinese base read on an English phone

- **WHEN** the phone's language is English and an entry has no English half
- **THEN** the entry shows its Chinese with a `zh` tag, and nothing on the sheet is hidden

### Requirement: The knowledge sheet marks a sensitive entry

A knowledge entry the API marks `sensitive` SHALL show "sensitive" on its row and in its
axes line, in the reader's language.

#### Scenario: A sensitive entry on the phone

- **WHEN** the knowledge sheet lists an entry with `sensitive: true`
- **THEN** the row's meta line starts with "sensitive ·" (「敏感 ·」) and the detail's axes
  line says it stays on the Mac

### Requirement: A plan window's bar is coloured by the core's tier

The usage sheet SHALL draw each plan window as a 6pt bar with round ends on a track that is
a pale wash of the fill's colour, keep at least a round dot for any non-zero value, and
write the figure as "N% used". The colour SHALL follow the window's `tier` from the core
and nothing else: blue for an ordinary window, amber for `warn`, the status red for `full`,
with the figure taking the amber or red. The phone SHALL NOT judge a percentage on its own;
a window from a serve too old to send `tier` SHALL be red at 100% and never amber.

#### Scenario: A week at its cap and a busy session

- **WHEN** the plan reads claude session 95% with no tier and claude week (fable) 100%
  with tier `full`
- **THEN** the session bar is blue and its figure reads in the page's ink, and the fable
  bar is red end to end with "100% used" in red

### Requirement: The usage sheet shows tokens by day

The usage sheet SHALL replace the per-session lifetime "output so far" block with a
tokens block: today's and this week's totals across every agent as two figures, a
seven-day bar chart (one neutral series, today in the stronger ink, direct labels on
today and the tallest day, weekday initials beneath), and the week's split per agent.
The block SHALL be absent when the serve carries no `history`.

#### Scenario: A week of work

- **WHEN** `history` reports seven days with today at 12.4M and the week at 69.0M
- **THEN** the sheet shows "12.4M today · 69.0M this week", seven bars with today's
  labelled, and one row per agent with its week total

### Requirement: A decision on the board can be given to HQ from the item itself

The situation board sheet SHALL render the commander's section (「还等你定的」 / "Still
waiting on you") as its numbered items, each a row under its group heading with a "Tell
HQ" affordance. Tapping an item SHALL offer "Do as you suggest" when the item carries a
recommendation, and "Let me say…" always. "Do as you suggest" SHALL send to HQ's pane a
reply that names the item by HQ's number and first line and accepts the recommendation;
"Let me say…" SHALL close the sheet and place that quote in the composer for the commander
to finish, opening the composer's field when it was resting (a quote placed in a field
nobody can see is not a hand-off). Without a handler the rows SHALL be read-only. A section with no numbered items
SHALL render as before.

#### Scenario: Taking HQ's recommendation

- **WHEN** the commander taps item 1 (「折中还是纯指路 —— 我建议折中…」) and chooses "Do as you suggest"
- **THEN** HQ receives 「态势板「还等你定的」第 1 条（折中还是纯指路 —— 我建议折中。…）：按你的建议办。」

#### Scenario: Saying it himself

- **WHEN** the commander taps item 5 (no recommendation) — only "Let me say…" is offered — and chooses it
- **THEN** the sheet closes, the composer's field opens, and it holds 「态势板「还等你定的」第 5 条（…）：」 with the cursor after it

### Requirement: A door on the HQ page shows one fact per line

Each of the three doors (board, knowledge, usage) SHALL show its value as one fact per
line, at most two lines, splitting the value at its " · " joints; a fact SHALL NOT end in
an ellipsis while a second line is free. With tokens by day available, the usage door's
value SHALL be today's and this week's totals; the tightest window is read in the usage
sheet behind it.

While a door's first fetch is still out, its tile SHALL be drawn in place with the
brand-mark loading placeholder where the value will be (not tappable); the tiles SHALL
NOT appear one by one as their fetches land. A door whose fetch settled with nothing to
show SHALL leave no tile.

#### Scenario: Opening the HQ page

- **WHEN** the page opens and the board, knowledge and usage reads are still in flight
- **THEN** three tiles are already there, each with the loading mark; each turns into its
  value as its read lands

#### Scenario: A phone-width tile

- **WHEN** the knowledge base holds 499 entries and 6 promotions wait on the commander
- **THEN** the tile reads "499 entries" over "6 waiting on you", both whole

### Requirement: The HQ page's composer rises above the keyboard

Opening the composer's field on the HQ page SHALL leave the field fully visible above the
software keyboard on every zone, the same as on a session's Detail.

#### Scenario: Typing to HQ on a phone

- **WHEN** the commander taps ⌨ on the HQ page, or a board item's "Let me say…" opens the field
- **THEN** the field, the key row and the chips sit above the keyboard; none is covered

### Requirement: HQ's recorded acts sit in the console, beside its words

The HQ page SHALL have two zones, "Your call" and "Console"; there SHALL be no zone of
its own for the supervisor's recorded acts (hq-work direction A, 2026-09-15: the tab said
in a fourth place what HQ's own words, the header's "HQ did" row and the Mac card already
said, with nothing to act on). The acts (`gtmux:audit:*` minus wake delivery) SHALL be
drawn in the console as small rows between the turns, each above the first turn that
came after it and the rest after the last turn: time, a dot (cyan for a dispatch or
reclaim, amber for an alarm, dim otherwise), the verb with its target, the detail, and
the worded outcome; a row that leads somewhere (a pane, a knowledge entry) SHALL open it
on tap. Three or more consecutive acts with the same verb inside one hour SHALL fold to
one row (「记账 ×6 ›」) that opens on tap. The header's "HQ did" row SHALL keep the day's
tally and lead to the console.

#### Scenario: A dispatch beside the claim

- **WHEN** HQ's reply at 23:36 says it sent %9 the commander's decision and the journal
  records the dispatch landing at 23:37
- **THEN** the console shows the reply bubble and, above the next turn, a row
  「23:37 · 派活 → %9 · 司令答了你那批问题里的第一条… · 已送达 ›」 that opens %9

### Requirement: HQ's work reads as sentences and leads somewhere

The "HQ's work" section SHALL open with one sentence saying what it is for (what HQ did
on the commander's behalf, listed to be checked; nothing needs handling). Each knowledge
act SHALL read as a sentence in the reader's language (recorded / rewrote / hit again /
promoted / landed / retired / …) with the journal's words kept for an unknown shape; a
dispatch's outcome SHALL be worded (landed / refused: a draft was in the box). A dispatch
or reap SHALL open that session on tap; a knowledge act SHALL open the knowledge base at
that entry.

#### Scenario: Checking what HQ recorded

- **WHEN** the journal holds `hit pitfalls/capture-pane-e-at-the-moment ×2`
- **THEN** the row reads 「又踩到：capture-pane-e-at-the-moment（第 2 次）」 and tapping
  it opens that entry in the knowledge base

### Requirement: The What's New popup folds older versions

When the popup spans versions, the newest version SHALL be open and every older version
SHALL be folded to its heading with an item count, opening in place on tap; the same
in Settings. A single-version popup SHALL show its items with no heading.

#### Scenario: Three skipped versions

- **WHEN** the popup holds 0.47.0 (6 items), 0.46.0 (5) and 0.45.0 (4)
- **THEN** 0.47.0's items are shown, 0.46.0 and 0.45.0 read as "0.46.0 · 5 items" and
  "0.45.0 · 4 items", and tapping 0.46.0 opens its five items in place

### Requirement: A paired Mac may answer at more than one address

The app SHALL fetch, on its first connection to a paired Mac and on every later one, the
addresses that Mac says it could answer at, and SHALL keep them with that Mac. When the
address it is using stops answering, it SHALL try the others, bounded the same way every
pairing step is, and SHALL save the one that answered as the address it uses from then on.

Only an address the Mac itself reported SHALL enter the list. A Mac is reported unreachable
only after every address in its list has been tried.

#### Scenario: The Mac moved to another server

- **WHEN** the saved address stops answering and another address the Mac reported does
- **THEN** the app connects through that one, saves it, and the user sees no more than the
  reconnect the app already shows

#### Scenario: None of the addresses answer

- **WHEN** every address for that Mac fails
- **THEN** the app reports it unreachable, with the same plain diagnosis it gives today,
  and keeps the addresses for the next attempt

#### Scenario: Addresses come only from the Mac

- **WHEN** the app refreshes a Mac's addresses
- **THEN** it stores what that Mac reported on an authenticated call and nothing else

### Requirement: The app shows which Direct server carries the Mac

The app SHALL show, with the connection state, WHERE that connection goes: the server's
name as a place, in the reader's language, when the Mac reports one. When there is nothing
to name (a local address, the standard tunnel) it SHALL show the address instead, as it
did before. While the Mac is unreachable it SHALL still name where it was last reached.

The app SHALL NOT offer to change servers. Moving cuts the connection the request would
travel through, so the result could not be observed from here; and the case that would want
it — the server is down while the user is away — is the case where the phone cannot reach
the Mac to ask at all. That choice lives on the Mac.

#### Scenario: A Mac on a named server

- **WHEN** the Mac reports the server it is on
- **THEN** the connection line reads as the state and that place ("Connected · Shanghai"),
  in the reader's language

#### Scenario: Nothing to name

- **WHEN** the Mac is reached over a local address or the standard tunnel
- **THEN** the connection line keeps showing the address, as before

#### Scenario: Unreachable

- **WHEN** the Mac cannot be reached
- **THEN** the line says so and still names where it was last reached

### Requirement: HQ's chat shows what is still running

In HQ's chat, above the composer, the app SHALL show one row when anything dispatched is
still running. It SHALL say how many are running, and SHALL say separately when one or more
of them is waiting on the user, because that is the state that needs a person. When nothing
is running the row SHALL NOT be rendered.

The row SHALL read as a control rather than as a status line: its own surface and a
chevron, in the same visual language as the approval card that already sits there. Colour
SHALL continue to carry state only, never tappability.

The row SHALL NOT appear in a worker pane's chat, because that pane is itself one of the
running things.

#### Scenario: Something is waiting on the user

- **WHEN** one dispatched task is waiting for input and two others are working
- **THEN** the row says so in two parts, with the waiting part in the waiting colour

#### Scenario: Nothing is running

- **WHEN** no dispatched task is running
- **THEN** no row is rendered and the composer sits where it always does

### Requirement: Opening the row lists the work and leads to it

Tapping the row SHALL open a sheet listing the tasks in two groups, running and finished,
with the finished group carrying its count and both collapsible. Each row SHALL carry the
goal, the agent, the pane, and how long the task has been in that state.

Tapping a row SHALL navigate to that task's pane. A task whose pane is gone SHALL be shown
dimmed and SHALL NOT offer navigation.

#### Scenario: Going to the work

- **WHEN** the user taps a running task in the sheet
- **THEN** the app opens that task's pane

#### Scenario: A task whose pane was closed

- **WHEN** a task's pane no longer exists
- **THEN** its row is dimmed, carries no chevron, and does not navigate
