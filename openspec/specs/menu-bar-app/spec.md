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
a 远程访问/Remote-access section (the door: Off / Local network / Anywhere), a
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

Every surface showing a pairing code (that sheet and the "Pair your phone" window)
SHALL keep it redeemable while it is open. A code expires after 5 minutes, works once,
and lives only in the serve's memory, so the surface SHALL replace it a minute before it
expires, after a device enrolls, and when the serve's `boot` (on `/api/health`) changes,
and SHALL NOT change the QR otherwise. Before this, a window minted one code when it
opened and showed it for as long as it stayed open; after an update restarted the serve
on 2026-09-19 the QR looked fine and every scan failed.

The Share section SHALL carry the consent master switch and the guest-link list —
each row showing the label, a scope summary (viewable count · typable count ·
expiry if any), created-at, and revoke — with a per-link inline scope editor (the
See/Type per-session columns) and a "新建分享/New share" sheet that names the link
AND selects its sessions in one step. Editing a link's scope SHALL affect ONLY
that link (the legacy global broadcast forms are not used by this UI).

#### Scenario: The reachability line catches up with the tunnel

- **WHEN** the "Pair your phone" window opens while the tunnel is still reconnecting (an
  update restarts it) and says it cannot reach the address
- **THEN** it checks again every 5 seconds and shows the address as reachable within
  one check of the tunnel answering, without the user pressing Refresh, keeping the
  last answer on screen in between
- **WHEN** the address is reachable
- **THEN** it is still checked every 30 seconds, so a tunnel that drops while the window
  is open is noticed

#### Scenario: A pairing QR outlives what kills its code

- **WHEN** a pairing window has been open for four minutes, or a device has just
  enrolled with its code, or the serve restarted and `/api/health` reports a new `boot`
- **THEN** within a few seconds the window shows a newly minted code, and a phone that
  scans it pairs
- **WHEN** none of those has happened
- **THEN** the QR stays exactly as it was

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

### Requirement: The supervisor's two memories are readable, and the knowledge base is judgeable, at the Mac

The HQ card SHALL offer two READERS, each in its own window rather than inside the
popover: the situation board, and the knowledge base with what it owes the commander at
the top. Both SHALL read exclusively through the CLI (`gtmux hq --board --json`,
`gtmux knowledge list --json`, `gtmux capture --list --json`), so the app stays a pure
consumer and never resolves the HQ home itself; that path is relocatable and symlinked on
real machines, so it SHALL be asked for (`gtmux hq --home`) rather than rebuilt.

The knowledge reader SHALL additionally offer the four JUDGMENT verbs — `promote`,
`land`, `retire`, `dismiss` — and SHALL NOT offer the AUTHORING verbs `add` and
`supersede`. The boundary this draws is between judging what is already written and
writing new prose, NOT between screens: DRIVING the fleet (send, spawn, deciding) stays
off the menu bar entirely, and nothing in either window dispatches anything.

Each act SHALL:

- run the CLI verb of the same name, from the HQ home, so the ledger — not a second
  implementation in the app — decides what the verb means and whether it is allowed;
- require a REASON, which the CLI requires anyway (`--why`, or `--ref` for `land`), and
  take exactly ONE confirmation naming the verb and its subject before running;
- report a failure as the CLI's own stderr, verbatim and unedited;
- refresh the window's contents on success, so the reader sees the state they created.

Which acts an entry offers SHALL follow the promotion lifecycle rather than being uniform:
a promoted-and-unlanded entry offers `land`, any other live entry offers `promote`, and
both offer `retire`. Candidates SHALL be grouped by dedup key, because `dismiss --capture
<key>` consumes every pending line sharing it.

Form and wording SHALL match the phone's knowledge sheet (MOBILE, `hq-knowledge-on-phone`)
rather than inventing a third dialect: `land` and `retire` reuse its copy, and the acts sit
in the entry's detail view rather than on the index rows a reader is scanning.

#### Scenario: Reading the board

- **WHEN** the commander opens the board reader and the supervisor has written one
- **THEN** the document is shown in a resizable window, selectable, laid out lazily so a
  50 KB board opens without a stall
- **AND WHEN** no board has ever been written
- **THEN** the window says so as an ordinary state, not as a failure

#### Scenario: The board's lists render as lists on the Mac as on the phone

- **WHEN** the board carries a numbered list whose items wrap onto indented lines
- **THEN** the Mac shows numbered items, each owning its wrapped lines, exactly as the
  phone does — never the items run together as one paragraph

#### Scenario: Closing out a promotion at the Mac

- **WHEN** the commander opens a promoted entry and confirms "mark it landed" with a ref
- **THEN** `gtmux knowledge land <id> --ref <ref>` runs from the HQ home, the ledger
  records it with the same `gtmux:audit:knowledge` trail any other door would leave, and
  the window re-reads so the entry has left the "waiting on you" section

#### Scenario: A refused act says what the CLI said

- **WHEN** an act is refused (an entry with no pending promotion, an unknown candidate
  key, a machine with no HQ home)
- **THEN** the CLI's own message is shown unedited, and nothing in the ledger changed

#### Scenario: A reason is required before anything runs

- **WHEN** the confirm sheet is open with an empty or blank reason
- **THEN** the confirming action is unavailable and no process is spawned

#### Scenario: Authoring is not offered here

- **WHEN** the commander looks for a way to add or supersede an entry
- **THEN** the window offers neither, and says the writing verbs stay in the CLI

### Requirement: The HQ window shows an entry's three axes and carries it

The knowledge tab SHALL show kind, provenance (with count) and audience on an entry as
the four audience words (中控 · 本机 · 仓库 · 全体 / hq · machine · repo · everyone), and
SHALL offer "write it in" for `hq` / `machine` / `repo` (preview, append, land) and
"feedback to gtmux" for `everyone`, plus `withdraw`. The pool view SHALL group by
neighbourhood.

#### Scenario: Carrying a machine-wide lesson

- **WHEN** the user confirms "write it in" on a `machine` entry
- **THEN** the canonical file and every agent block are refreshed, the entry lands with
  the canonical path as ref, and doctor's sync row is green

### Requirement: The knowledge window follows the app's language for content too

The knowledge window SHALL show each entry's half matching the app's language setting,
fall back to the other half with a small language tag, and match search against both.

#### Scenario: Switching the app to English

- **WHEN** the language setting changes to English while the knowledge window is open
- **THEN** entries with an English half re-render in English without a refetch, the rest
  show Chinese with a `zh` tag

### Requirement: The HQ card expands into the chief-of-staff report

The HQ card SHALL carry a disclosure at the right end of its head that opens, inside the
card's bordered panel, the same report table the phone's HQ page expands into (MOBILE
§17): a key column and a value column, one row per question — `machine` (readings; a door
to the reader's machine tab) · `knowledge` (entry count, what it owes the commander and
the oldest debt; a door) · `board` (how fresh; a door) · `usage` (today's and this week's
tokens when the CLI carries `history`, then one window per plan, the tightest, from
`gtmux usage --json`; a door to the reader's Usage tab; absent when neither is readable) · `HQ did` (the last day's tally of
the supervision's own acts, from `gtmux events --since 24h --acts`, in the phone's fixed
order). The head's click SHALL still focus the supervisor's pane. Keys, values and verbs
SHALL follow the app's language.

Rows SHALL follow the phone's rules: a row with nothing to say is absent, never a zero;
the machine row leads, in the attention colour and allowed a second line, ONLY at the red
tier, keeps its ordinary place in amber at the amber tier, and is plain otherwise; the
knowledge row alone may turn amber, and only when the oldest promotion has waited past the
two-week line `gtmux doctor` uses.

The expansion SHALL open itself once on ENTERING an attention state (the supervisor
waiting, a worker waiting, or a red resource tier) and never close itself; a manual toggle
SHALL be remembered across popover openings. Its data SHALL be read through the CLI only
while the expansion is showing in an open popover, and the events read SHALL run from the
app's own working directory, never the HQ home, so it cannot advance HQ's consumption
watermark.

#### Scenario: A red machine explains itself on the card

- **WHEN** `gtmux resource --json` reports `machine.tier` = `red` while the card was
  collapsed
- **THEN** the card opens itself, the machine row is first and red, and it reads the
  memory, disk, load and reclaimable-orphan figures, with a door to the machine tab

#### Scenario: The knowledge debt is on the card

- **WHEN** 7 promotions await the commander and the oldest has waited 16 days
- **THEN** the knowledge row reads the entry count, "7 waiting on you" and "oldest 16d"
  (「7 条待你带走 · 最久 16 天」 in Chinese) in amber, and opens the knowledge tab

#### Scenario: All normal stays one line

- **WHEN** nothing is waiting, the machine is healthy and the reader has not opened the
  report
- **THEN** the card shows the medallion and the headline only, with the disclosure closed

### Requirement: The reader window shows usage

The HQ reader window SHALL offer a Usage tab reading `gtmux usage --json`: the tightest
non-session window and its reset as a lead, quotas grouped by agent as bars coloured by
each window's tier (blue, amber when the core warns, red at 100%, with no separate warning
line),
per-agent output with the note that it is not a billing period, and sessions sorted by
trouble (alerted, then burn rate, then context share) with parked sessions folded into a
count. It SHALL poll only while it is the showing tab.

#### Scenario: Where am I standing

- **WHEN** the plan windows read claude session 30%, claude week (all models) 29%, claude
  week (fable) 49%, codex week 0%
- **THEN** the card's usage row reads "claude Fable 49% · codex wk 0%" and the Usage tab
  leads with the fable week at 49%

### Requirement: The reader window shows the machine

The HQ reader window SHALL offer a third tab, Machine, reading `gtmux resource --json`:
the four readings (memory, disk, load, power), the core's own warning sentence, the
per-agent RSS/CPU table heaviest first with each pane's session name, and the orphan
processes the core calls reclaimable with the core's own hint. The tab SHALL be read-only:
it SHALL offer no kill or reclaim action. It SHALL poll only while it is the showing tab.

#### Scenario: Reading a red tier

- **WHEN** the machine is at the red tier because memory is critical
- **THEN** the machine tab shows the memory reading marked critical, the core's warning
  sentence, and the heaviest agent process in the attention colour, with no button that
  ends a process

### Requirement: The Mac export asks for its lock first, then the place, then confirms

The reader window's Export… SHALL open a sheet that asks for the passphrase (two fields, a
show toggle, and one hint line naming the one thing to fix) before the save panel, SHALL
offer to keep the passphrase in this Mac's keychain so the next export does not ask, SHALL
show a remembered passphrase as a single line with a way to change it, SHALL hand the
passphrase to the CLI over stdin and never on the command line, and SHALL end on a
confirmation page carrying the written path, its size, and how the file opens — or, on
failure, the CLI's own words. The memory line SHALL show when the last export was made and
whether it was locked.

#### Scenario: First export on a Mac

- **WHEN** the commander clicks Export… with no passphrase in the keychain, types one of
  twelve characters twice and leaves "remember" on
- **THEN** the hint reads "good", the save panel offers `gtmux-hq-<date>.tar.gz.age`, the
  file is written locked, the passphrase is stored in the keychain, and the sheet ends on
  the path, the size and the sentence saying how it opens

#### Scenario: Second export

- **WHEN** the commander clicks Export… again
- **THEN** the sheet shows one line saying the keychain's passphrase will be used, with
  "Change…", and the export is one click away

### Requirement: The knowledge window marks a sensitive entry

A knowledge entry the API marks `sensitive` SHALL show a lock on its row and "sensitive ·
this Mac only" in its axes line; an attempt to promote it past `hq` SHALL show the CLI's
refusal verbatim.

#### Scenario: A sensitive entry in the list

- **WHEN** the knowledge window lists an entry with `sensitive: true`
- **THEN** its row carries a lock and its detail's axes line says it stays on this Mac

### Requirement: The reader's Usage tab shows tokens by day

The Usage tab SHALL show, in place of the per-agent lifetime totals, the same tokens
block as the phone: today's and this week's totals, a seven-day bar chart with today in
the stronger ink and direct labels on today and the tallest day, and the week's split per
agent; absent when the CLI carries no `history`.

#### Scenario: The same week on the Mac

- **WHEN** `gtmux usage --json` reports the week above
- **THEN** the Usage tab's tokens block reads the same two figures and draws the same
  seven bars

### Requirement: The pairing window explains an unreachable address from tunnel status

When the "Pair your phone" window cannot reach its own pairing address, it SHALL explain
why from `status/tunnel.json`, for either backend: connected and fresh means this Mac
cannot see its own address but the tunnel is up and a phone on cellular connects; down
means no device connects, shown with the recorded error; stale or missing means it cannot
tell and SHALL say only that it cannot reach the address yet. It SHALL NOT read any log to
decide. The pairing sheet's access bar SHALL show the tunnel's reported state under
Anywhere, and nothing when the status is stale.

#### Scenario: Direct on a network that hijacks DNS

- **WHEN** the backend is Direct, the window's probe fails, and `status/tunnel.json`
  reports `down` with a resolver error
- **THEN** the window says no device can connect and shows that error, and does not tell
  the user that a phone on cellular connects

#### Scenario: Standard, visible to the phone but not to the Mac

- **WHEN** the window's probe fails and `status/tunnel.json` reports `connected` and is
  fresh
- **THEN** the window says this Mac cannot reach its own address but a phone on cellular
  connects

### Requirement: The menu bar app writes to the gtmux log store

The menu bar app SHALL write its entries to the gtmux log store in the shared schema and
under the same redaction, and SHALL mirror them to the unified log under subsystem
`com.gtmux.menubar`: its start, the notifications it shows or does not show (actor
`menubar`), failures (a gtmux command it ran, a pairing-code mint, an update) with their
exit or HTTP status, and each change of the pairing window's reachability verdict. The
gtmux commands and serve requests it makes SHALL name it as their actor
(`GTMUX_ACTOR=menubar`, `X-Gtmux-Actor: menubar`), so what gtmux does for it is recorded
as the menu bar's act. `GTMUXBAR_DEBUG` SHALL add debug entries.

#### Scenario: A pairing code could not be minted

- **WHEN** the menu bar fails to mint a pairing code
- **THEN** the store gains a `mint.failed` entry from component `menubar` with the HTTP
  status or error, and `gtmux logs --component menubar` shows it

### Requirement: The empty panel offers the two ways out of it

With no agents on the radar, the popover SHALL show, left-aligned and in this order: what
it is waiting for (one line, plus one line saying an agent appears here with what it is
waiting for), the launch command with a button that copies it to the pasteboard and
confirms briefly, one line naming the agents gtmux recognises, and then a "New session"
row in the agent list's row language. The restore row SHALL follow directly below it when
there is a working set to come back to, so the panel offers two comparable rows rather
than a button and a banner. The app mark SHALL NOT be repeated in the body; the header
already carries it.

#### Scenario: A fresh install has one door

- **WHEN** the radar is empty and the restore plan came back empty
- **THEN** the restore row is absent and the New session row is the only action in the
  panel's body

#### Scenario: Copying the command

- **WHEN** the user clicks Copy beside the launch command
- **THEN** the command is on the pasteboard and the button says so until it reverts

