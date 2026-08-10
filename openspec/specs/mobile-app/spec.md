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
