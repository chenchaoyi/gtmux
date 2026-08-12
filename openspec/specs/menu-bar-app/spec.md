# Menu-Bar App Specification

## Purpose

An always-visible macOS menu-bar app that shows, at a glance, the most-urgent
agent state and a popover list grouped by who needs you. It is a pure consumer of
the CLI (polls `gtmux agents --json`, shells out to `gtmux focus`) and the click
target for notifications.
## Requirements
### Requirement: Ambient status item

The system SHALL render an `NSStatusItem` whose glyph encodes the most-urgent
state by COLOR — the brand-grid mark tinted to the state's palette color
(waiting → working → idle → calm) — with a count badge of the most-urgent
actionable count. (Since the 2026-06 UI overhaul, #160, the glyph is color-only:
one tinted brand mark for every state, NOT a per-state shape.)

#### Scenario: Most-urgent wins

- **WHEN** at least one agent is waiting
- **THEN** the status item's brand mark is tinted the waiting (red) color and shows
  the waiting count badge

### Requirement: Grouped popover

The system SHALL show a popover listing agents grouped in fixed order
waiting → working → idle → running, only non-empty sections, each row carrying
the agent avatar + status badge + session/task, with the waiting section
emphasized.

#### Scenario: Jump from a row

- **WHEN** a row is clicked (or Enter / ⌘1–9)
- **THEN** the app runs `gtmux focus <pane>` and lands on that agent

### Requirement: The popover grows with the fleet and stays on screen

The popover SHALL size its agent list to the list's measured content height, capped
so that the whole panel — list plus its measured chrome (header, HQ card, footer,
update banner) — fits the display holding the menu bar. The panel's height SHALL be
reported to the popover itself, because `NSPopover` positions its window from
`contentSize` and a SwiftUI view that resizes itself never updates that property.

#### Scenario: A dozen agents

- **WHEN** the fleet has more rows than fit a short panel and the display has room
- **THEN** the popover is tall enough to show them without scrolling, and its top
  edge stays below the top of the display

#### Scenario: More rows than the display can hold

- **WHEN** the list's content is taller than the room left by the chrome
- **THEN** the list is capped at the remaining room and scrolls inside it; the panel
  still fits the display

#### Scenario: A resize leaves the open panel off screen

- **WHEN** the panel is resized while open and the result is positioned outside the
  display
- **THEN** the app re-attaches it to the status item; a panel that is merely tall
  enough to touch the top of the display is left alone

### Requirement: Pure CLI consumer

The system SHALL source all data from `gtmux agents --json` and SHALL NOT
duplicate detection logic; gtmux-core stays the single data source.

#### Scenario: Poll for updates

- **WHEN** the refresh timer fires
- **THEN** the app re-runs `gtmux agents --json` and repaints

### Requirement: Notification click target

The system SHALL be the notification target (`com.gtmux.menubar`): it drains the
notify queue, posts native banners, and on click jumps to the last-finished
agent.

#### Scenario: Click a banner

- **WHEN** the user clicks a delivered notification
- **THEN** the app activates and runs `gtmux focus --last`

### Requirement: Menu bar shows a distinct native-sessions category
The menu-bar popover SHALL group `source: "native"` sessions under their own labelled section (e.g. "Elsewhere" / "不在 tmux"), separate from the tmux-based needs-you / working / idle groups, so users can see these sessions exist and their rough info (agent, project, state, idle time) without implying they can be jumped to or replied to.

#### Scenario: Native section rendered when native sessions exist
- **WHEN** the app polls `agents --json` and native sessions are present
- **THEN** they SHALL appear in a dedicated, clearly-labelled category distinct from the tmux groups

#### Scenario: Native rows expose no jump or reply affordance
- **WHEN** a native row is rendered
- **THEN** it SHALL NOT show a jump chevron or a reply/send control, and clicking it SHALL NOT attempt a terminal focus

### Requirement: Move-to-tmux action in the menu bar
The menu bar SHALL provide a "Move to tmux" action on an eligible native row that resumes that conversation in a fresh tmux session. The action SHALL be shown only for a row that is movable (idle, resumable, with an on-disk conversation), and SHALL surface a confirmation explaining that the original process is exited before acting.

#### Scenario: Move a native session
- **WHEN** the user triggers Move to tmux on a movable native row and confirms
- **THEN** the app SHALL invoke the resume/spawn path to open a tmux session running that conversation

#### Scenario: Move hidden for ineligible rows
- **WHEN** a native row is not movable (working, non-resumable, or no on-disk conversation)
- **THEN** the Move to tmux action SHALL NOT be offered for that row

### Requirement: Mark errored idle rows in the popover

The menu-bar popover SHALL visually distinguish an idle agent that ended on an
error (`error: true` in the `agents --json` contract) from a successfully-finished
idle agent, using an amber ⚠ "errored" modifier and the `error_text` summary in
place of the green ✓. The row SHALL remain in the IDLE section and MUST NOT use the
red `waiting`/needs-you color.

#### Scenario: Errored idle agent

- **WHEN** an agent row has `status: idle` and `error: true`
- **THEN** the popover renders it in the IDLE section with an amber ⚠ marker (not
  the green ✓) and shows the `error_text` summary
- **AND** it is not colored red and does not sort into NEEDS YOU

#### Scenario: Successful idle agent unchanged

- **WHEN** an agent row has `status: idle` without `error`
- **THEN** the popover renders it exactly as today (green ✓)

### Requirement: Check for updates + one-click self-update

The app SHALL check for a newer release (reusing the CLI's own `gtmux update --check`)
and offer a one-click update that reuses `gtmux update` (CLI + app), spawned DETACHED
so it survives the installer pkill'ing + relaunching the app.

The one-click update SHALL ALWAYS terminate in a defined state — **relaunched to the
new version**, or an **`updateFailed` retry** — and SHALL NOT sit on the "Updating…"
spinner forever. Concretely:

- The installer SHALL relaunch the swapped app with a **force-new-instance** launch
  (`open -n`), never a bare `open` that can re-activate a not-yet-exited old instance
  instead of launching the freshly-swapped binary. The app's newest-wins
  single-instance guard SHALL terminate any older instance so no duplicate status item
  remains.
- The detached job records its exit code. A **non-zero exit** (network blip / SHA
  mismatch) SHALL flip to `updateFailed` with a retry.
- On a recorded **exit 0**, the installer is expected to have pkill'd + relaunched the
  app; if the app is nonetheless STILL running past a short grace period, the relaunch
  did not take, and the app SHALL self-heal by comparing the on-disk installed bundle
  version to its own running version:
  - **installed version newer than running** → the swap succeeded but the relaunch was
    missed; the app SHALL force-launch the installed bundle (`open -n`) and terminate
    itself, so the new version takes over.
  - **installed version equal to running** (or unreadable) → the swap never happened
    (e.g. the app download was skipped); the app SHALL flip to `updateFailed` with a
    retry rather than spin.
- A download that wedges BEFORE any exit code is recorded SHALL still be caught by a
  hard timeout that flips to `updateFailed`.

#### Scenario: Update fails and offers retry

- **WHEN** a one-click update's download fails (network blip / SHA mismatch)
- **THEN** the app flips to an "update failed — retry" banner (not a stuck spinner),
  and tapping it re-runs the update

#### Scenario: Installer relaunch is missed but the swap succeeded

- **WHEN** the detached `gtmux update` records exit 0, but this app is still running
  past the grace period AND the on-disk `Gtmux.app` bundle version is newer than the
  running version
- **THEN** the app force-launches the installed bundle with `open -n` and terminates
  itself, so the newer version takes over (rather than spinning on "Updating…")

#### Scenario: Installer reported success but the app was never swapped

- **WHEN** the detached `gtmux update` records exit 0, but this app is still running
  past the grace period AND the on-disk `Gtmux.app` bundle version equals the running
  version (the app step was skipped)
- **THEN** the app flips to an "update failed — retry" banner rather than spinning on
  "Updating…"

### Requirement: Right-click to quit

The status item SHALL expose a right-click (secondary-click) context menu with a Quit
action, so the app can be quit without going through the popover.

#### Scenario: Right-click Quit

- **WHEN** the user right-clicks the status item and chooses Quit
- **THEN** the app terminates

### Requirement: Background-running idle modifier in the popover

An idle row whose settled turn left in-flight background work SHALL carry a
background-running modifier in the popover (matching the radar/`agents --json` `bg`
fields), so a "done but a background task is still running" session is distinguishable
from a fully-finished one.

#### Scenario: Idle row with background work

- **WHEN** an idle agent's `agents --json` row carries the `bg` marker
- **THEN** its popover row shows the background-running modifier alongside the idle badge

### Requirement: The supervisor renders as its own layer (HQ card)

The popover SHALL render a supervisor session (`role:"supervisor"`) as a
persistent compact card between the header summary and the grouped section list
— NEVER as a row inside the waiting/working/idle/running sections (those rows
SHALL exclude supervisor rows). The card SHALL be visually framed so it does NOT
read as one more session: a ROLE BANNER above it (a small uppercase
"CHIEF OF STAFF / 参谋长" label with an oversight glyph and a short purpose line,
e.g. "watches all sessions / 统观全局") and a BORDERED panel (agent rows carry no
border — the border is the primary "not a row" cue).

The card's avatar SHALL be a circular **HQ medallion** — the gtmux brand pane-grid
mark plus an "HQ" wordmark inside a ring — that is the SAME visual token as the mobile
HQ disc (MOBILE §17), so the supervisor reads as one identity across surfaces. The
medallion's RING COLOR and a small corner BADGE SHALL encode the full HQ state model,
in parity with the mobile disc and resolved by the same priority order (a pure resolver
mirroring the mobile `discState`):

- **needs-your-call** — the supervisor itself is `waiting` → RED ring + a `!` badge.
- **worker-needs-you** — ≥1 non-supervisor session is `waiting` → RED ring + the waiting
  COUNT as the badge.
- **resource-bottleneck** — the machine is at the `red` resource tier (a genuine
  bottleneck; from a slow `gtmux resource --json` poll of `machine.tier`, NOT a soft
  amber) → RED ring + a `⚠` badge.
- **working** — the supervisor is `working` → CYAN ring, no badge.
- **normal** — all quiet → GREEN ring, no badge.

RED is reserved for "needs attention" (a decision or a genuine bottleneck); the badge
disambiguates which. The ring/badge colors SHALL be the authoritative status palette
(DESIGN §9 / `Theme.Status`), the same values the section badges use — the medallion
adds NO new colors. The card SHALL still carry the deterministic INTELLIGENCE HEADLINE
as its subtitle (the chief-of-staff conclusion: who needs you + how many others are
normal, or "all normal"); the ring/badge is the at-a-glance layer and the headline is
the sentence. Clicking the card focuses the supervisor's pane (the command console
lives on mobile/web, not the menu bar).

When no supervisor is live, the slot SHALL show a quiet grey (dimmed) medallion with a
"not running — start" affordance that launches `gtmux hq` (the app stays a CLI
consumer).

#### Scenario: Supervisor live

- **WHEN** an `agents --json` row carries `role:"supervisor"`
- **THEN** the popover shows the HQ card (the HQ medallion + intelligence headline)
  above the sections, and that row does NOT appear inside any section

#### Scenario: The medallion ring encodes state in parity with the mobile disc

- **WHEN** the supervisor is `working` and nothing is waiting and the machine is not at
  the red tier
- **THEN** the medallion ring is CYAN with no badge
- **AND WHEN** a non-supervisor session is `waiting`, the ring is RED with the waiting
  count as the badge
- **AND WHEN** the supervisor itself is `waiting`, the ring is RED with a `!` badge,
  outranking a waiting worker and a resource bottleneck

#### Scenario: A soft resource amber does not redden the medallion

- **WHEN** `gtmux resource --json` reports `machine.tier` = `amber` (a soft heads-up,
  not a bottleneck) and nothing is waiting
- **THEN** the medallion stays on the supervisor's own state (working / normal) — only
  a `red` tier drives the resource state (低噪, matching the mobile disc)

#### Scenario: Supervisor absent

- **WHEN** no row carries `role:"supervisor"`
- **THEN** the HQ slot shows the quiet grey medallion start affordance, and clicking it
  shells `gtmux hq`

### Requirement: Shared-input control surface

The menu-bar app SHALL provide a host control surface for web-shared VIEW and INPUT that
mirrors `gtmux share`, so the host can consent to and scope both what a guest SEES and
what a guest TYPES into without dropping to a terminal. The controls SHALL live in a
"Shared input" section of Preferences, beside Remote access (guests arrive over the same
serve/tunnel):

- a **consent toggle** (default reflecting the current state; OFF by default), which
  turns shared input on/off;
- a **per-pane allowlist** rendered from the live agent list — each tmux pane
  (`source == "tmux"`, a real `%N`) a row with TWO independent controls: 👁 **can-see**
  (adds the pane to the guest VIEW allowlist) and ⌨️ **can-type** (adds it to the INPUT
  allowlist). The can-type control SHALL be DISABLED unless can-see is on for that pane
  (input ⊆ view). Each row SHALL carry the SAME identity the session list shows — the
  agent avatar (official icon + state), the agent's own session title (`primary`), and a
  dim `session · %pane` line — ordered like the radar (state rank → session title), so
  the host controls the pane they RECOGNISE from the popover;
- **guest share links**: existing links listed with a per-link revoke, and a "new share
  link" action that mints a link and copies its URL to the clipboard.

The app SHALL remain a pure CLI consumer: it MAY read the local `share.json` for the
consent/view/input state, but SHALL perform every mutation by invoking `gtmux share …`
(including `gtmux share view add/remove %N` for the view controls), and SHALL obtain the
guest list and minted URL from the CLI's token-free `--json` output. The server gate
stays authoritative; the app only reflects and drives it.

When shared input is LIVE (consent on AND at least one input-allowed pane AND at least one
guest link), the popover SHALL show a quiet exposure indicator — a type-into-terminal
exposure is never silent, the same ethos as the "Remote on" indicator.

#### Scenario: Host consents and allows a pane from the menu bar

- **WHEN** the host ticks 👁 can-see on a tmux pane row, then ticks ⌨️ can-type on it
- **THEN** the app invokes `gtmux share view add %N` then `gtmux share add %N`, and the row reflects both — that pane is now guest-viewable and (with consent on) guest-typable

#### Scenario: Can-type is gated by can-see

- **WHEN** a pane's 👁 can-see is off
- **THEN** its ⌨️ can-type control is disabled; turning can-see off on a pane that was typable also clears its can-type (input ⊆ view)

#### Scenario: Allowlist rows carry the session-list identity

- **WHEN** the host opens the Shared-input allowlist while several same-agent (e.g. all Claude Code) tmux panes are live
- **THEN** each row shows that pane's own session title (`primary`) with the agent avatar and a dim `session · %pane`, matching the popover's session list — the rows are distinguishable by session, not a generic agent name repeated with only a raw `%N` to tell them apart

#### Scenario: Minting a share link copies it

- **WHEN** the host taps "new share link"
- **THEN** the app invokes `gtmux share new --json`, shows the resulting URL, and copies it to the clipboard for the host to send to a collaborator

#### Scenario: Revoking a link from the menu bar

- **WHEN** the host taps revoke on a listed guest link
- **THEN** the app invokes `gtmux share revoke <id>`, exactly that link stops working, and it disappears from the list

#### Scenario: Live shared input is not silent

- **WHEN** consent is on, at least one pane is input-allowed, and at least one guest link exists
- **THEN** the popover shows a compact shared-input exposure indicator that opens Preferences when tapped

### Requirement: Preferences present the two-track pair/share model

The Preferences window SHALL organize remote capability into the two-track model:
a 远程访问/Remote-access section (the door: Off / Wi-Fi / Anywhere), a
你的设备/Pair section, and a 分享/Share section — so "my own surfaces" and
"collaborator access" never mix.

The Remote-access section is the SHARED reachability door: BOTH paired (owner) devices
AND shared (guest) collaborators reach the Mac through it, so it SHALL be its OWN
section (not nested under the Pair roster) — its settings govern pair and share alike.

When Anywhere is on, the Remote-access section SHALL surface which TUNNEL BACKEND is
active (Standard = the zero-config hosted tunnel, vs Direct = the user's own VPS +
domain), and — when Direct is configured on this Mac — SHALL offer a Standard | Direct
switch that changes the backend, so the choice the CLI's `gtmux tunnel --backend`
already exposes is not hidden behind an opaque "Anywhere". The backend governs both
pair and share URLs (both ride the same tunnel), so it lives in the door section, not
the Pair section.

The Pair section SHALL list paired (owner-scope) devices — name, a kind icon,
last-seen, and per-row revoke — plus a single "配对新设备/Pair a device" action
opening one sheet that renders the SAME enroll code in the three media (phone QR /
browser URL+code / terminal attach one-liner).

The Share section SHALL carry the consent master switch and the guest-link list —
each row showing the label, a scope summary (viewable count · typable count ·
expiry if any), created-at, and revoke — with a per-link inline scope editor (the
See/Type per-session columns) and a "新建分享/New share" sheet that names the link
AND selects its sessions in one step. Editing a link's scope SHALL affect ONLY
that link (the legacy global broadcast forms are not used by this UI).

#### Scenario: Pair and Share never mix

- **WHEN** the user opens Preferences with two paired devices and two share links
- **THEN** the devices appear only under Pair and the links only under Share, each
  with its own list styling and actions

#### Scenario: Anywhere surfaces its tunnel backend

- **WHEN** Anywhere is on and Direct is configured on this Mac
- **THEN** the Remote-access section shows a Standard | Direct switch reflecting the
  active backend, and choosing one re-runs the tunnel on that backend
- **WHEN** Anywhere is on and Direct is NOT configured
- **THEN** it shows that Standard (hosted) is active and how to set up Direct, rather
  than hiding the backend entirely

#### Scenario: A share is created with its scope in one step

- **WHEN** the user clicks 新建分享, names it "Alice", ticks session A as
  See+Type, and confirms
- **THEN** one link is minted whose scope is exactly that selection, the URL is
  copied/surfaced, and other links' scopes are untouched

#### Scenario: Per-link editing touches one link

- **WHEN** the user expands link "Alice" and unticks a session's Type
- **THEN** only Alice's input allowlist changes; other links and the template are
  unaffected

### Requirement: HQ card shows an intelligence headline, not fleet pips

The menu-bar HQ (chief-of-staff) card SHALL NOT render a row of per-worker "fleet pips"
(they duplicate the section list and the summary count, and are anonymous). Its subtitle
SHALL be a deterministic intelligence headline synthesized from the worker fleet: when a
worker is waiting, it names the one that needs the user plus a count of the rest that are
normal; when nothing is waiting, it reads as "all normal, nothing needs you". The
headline is coloured for attention (red/amber) when a worker or HQ itself needs the user,
and dim when quiet.

#### Scenario: A worker is waiting

- **WHEN** the fleet has one or more waiting workers
- **THEN** the HQ card subtitle names the first waiter and how many others are normal (e.g. "api needs you · 4 others normal"), with attention colour — and shows no pip row

#### Scenario: All quiet

- **WHEN** no worker is waiting
- **THEN** the HQ card subtitle reads as "all normal — nothing needs you", dim, with no pip row

### Requirement: Popover width sized for content legibility

The menu-bar popover SHALL use a fixed content width (a single design token,
`Theme.Size.popoverWidth`) wide enough that the digest text — the HQ card's
goal/last/ask line and each agent row's session/task line — is legible before
tail-truncation. The width SHALL be **420pt**, matching a companion menu-bar app so the
two menu-bar apps read as one visual family. Every row SHALL inherit this width (via
the popover frame or `maxWidth: .infinity`); no per-row width may be hardcoded.

Long content SHALL be handled by single-line tail-truncation, NOT by reflowing the
popover: the goal/last/ask and session/task lines SHALL remain
`lineLimit(1)` + `truncationMode(.tail)` at any width, so the wider frame only reveals
more text and never changes the number of lines.

The width SHALL be a fixed constant rather than content-adaptive: because every row is
single-line tail-truncated, no row's content requires a wider frame, so an adaptive /
max-width popover would add width jitter with no legibility gain.

#### Scenario: Popover renders at the calibrated width

- **WHEN** the popover is shown
- **THEN** its content frame is 420pt wide
- **AND** the width comes from the single `Theme.Size.popoverWidth` token, not a
  per-row constant

#### Scenario: A long goal/last/ask line truncates rather than reflows

- **WHEN** an HQ card or agent row carries a goal/last/ask or session/task string
  longer than the row can show
- **THEN** the string is shown on one line, tail-truncated with an ellipsis
- **AND** the wider frame reveals more of the string but does not add a second line
  or change the popover width

### Requirement: Server mode marks the existing glyph, and only Preferences changes it

While server mode is on, the app SHALL indicate it by modifying the EXISTING menu-bar
glyph and SHALL NOT add a second status item. Two icons read as two applications; one
icon in a different state reads as the same application doing something.

The indication SHALL borrow the recording-indicator language — a small lit dot that
breathes slowly — because that is the one visual convention users already read as "this
is still running, you left it on", and being forgotten is this feature's central risk.
This is a deliberate exception to the rules that colour encodes agent state and that the
product animates only once, and it SHALL be bounded so it cannot be mistaken for a
waiting agent: the dot SHALL be small and sit ON the mark (a waiting agent turns the
WHOLE mark, a different silhouette at a glance), its motion SHALL be slow and shallow
enough to read as "alive" rather than "alarm", and it SHALL stay legible when the mark
beneath it carries the waiting colour.

The animation SHALL exist only while server mode is on — no timer and no repainting when
it is off. Server mode SHALL NOT appear as a row or section in the agent radar.

Every surface MAY show this state; only Preferences SHALL be able to change it, because
every path to changing it ends at an administrator password typed at the machine. The
popover MAY state it alongside the agent summary, read-only.

#### Scenario: On and visible without adding an icon

- **WHEN** server mode is on
- **THEN** the existing glyph carries a slowly breathing dot, the menu bar gains no
  additional status item, and the glyph's own colour still reflects only the agent state

#### Scenario: Not confusable with an agent waiting

- **WHEN** server mode is on and an agent is also waiting
- **THEN** the mark carries the waiting colour AND the dot remains distinguishable on top
  of it, so both states are readable at once

#### Scenario: Off costs nothing

- **WHEN** server mode is off
- **THEN** no animation timer is running and the glyph is drawn exactly as it was before
  the feature existed

### Requirement: The server-mode indicator encodes state by shape, colour only for attention

The indicator SHALL carry a distinct glyph that reads as "this machine is being kept
awake", and SHALL be rendered in the neutral palette colour (`#8E8E93`) while healthy,
turning the authoritative waiting red (`#EF4444`) only when a guardrail wants the user
(running on battery under an override, an unhealthy guard, a thermal advisory). It SHALL
NOT introduce a new colour token, gradient, or glow, and SHALL NOT animate. Shape carries
the presence signal; colour is reserved for attention, exactly as it is everywhere else in
the product.

#### Scenario: Healthy versus wanting attention

- **WHEN** server mode is on and healthy, and then a guardrail trips
- **THEN** the indicator is neutral in the first case and the authoritative red in the
  second, with the reason stated in its menu, and in neither case does it animate or use a
  colour outside the palette

### Requirement: The administrator prompt is explained before it appears

Before triggering the macOS administrator dialog, the app SHALL show a plain-language card
that states: what changes (one system power setting), that server mode stays on until the
user turns it off, that a persistent menu-bar indicator will be present the whole time and
can end it, that a de-escalation-only guard is installed in the same authorization and
removes itself, that it will not run on battery and ends if the machine is unplugged, that
a closed lid dissipates heat worse, and that the machine stays remotely reachable for the
duration. The copy SHALL follow the design system's first-run tone rule — factual, no
marketing phrasing — and SHALL be provided in both English and Chinese. Server mode SHALL
be manageable from Preferences ONLY, in its own titled section placed before the
remote-access, pairing and sharing sections — those three form one continuous run about
who may reach the machine and SHALL NOT be split. The section SHALL show the live state,
how long it has been on, charge when on battery, guard health, and the platform verdict,
and SHALL offer an explanation of what server mode is for someone meeting the term for
the first time.

#### Scenario: First enable

- **WHEN** the user turns server mode on from the menu bar for the first time
- **THEN** the explainer card appears before any system dialog, states that it stays on
  until switched off, and declining it leaves every system setting unchanged
