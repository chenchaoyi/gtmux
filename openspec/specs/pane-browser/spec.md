# pane-browser Specification

## Purpose
Enumerate tmux panes for a separate session/window/pane browser, so plain shells
and editors remain reachable without crowding the coding-agent radar. Define the
tier-specific controls, opt-in watch promotion, and shared browsing requirements
across the menu bar, phone, iPad, Web, and CLI.

## Requirements

### Requirement: Enumerate all tmux panes as a read-only producer

gtmux SHALL provide a read-only command `gtmux panes` that enumerates every tmux
pane across all sessions and windows, with `--json` emitting a structured array and
no arguments printing a human-readable session → window → pane tree. Each pane entry
SHALL carry at least: pane id, locator (`session:window.pane`), session name, window
index, pane index, working directory, current command, title, active flag,
copy-mode flag, and a `tier` field of `"agent"` or `"plain"`. The command SHALL have
NO side effects (it only reads tmux) and SHALL NOT alter, reorder, or filter the
`gtmux agents --json` contract — `agents` remains the coding-agent radar; `panes` is
the superset that also includes plain shells.

#### Scenario: Reading the panes changes nothing

- **WHEN** the pane list is read (`gtmux panes`, `--json`, or `GET /api/panes`)
- **THEN** no file of gtmux's state is created, changed or removed by the read: no turn
  or watched marker of a closed pane is swept, no native record is pruned, no screen or
  CPU baseline is sampled and no icon is cached; an agent pane's tier, name, icon and role
  are the ones the radar gives it, the icon path included even while the radar has not
  yet cached it

#### Scenario: A window with an agent and a plain shell

- **WHEN** a window holds one pane running a coding agent and one bare shell pane, and
  `gtmux panes --json` is run
- **THEN** both panes appear, the agent pane has `tier:"agent"` and the shell pane has
  `tier:"plain"`, and `gtmux agents --json` still lists only the agent pane

### Requirement: Plain tmux panes are first-class control targets

A pane whose tier is `plain` SHALL be focusable, typeable, and attachable exactly as
an agent pane is — the underlying `gtmux focus`/`send`/`attach` primitives act on any
pane id. Surfaces SHALL apply the tier to decide what to OFFER: a plain pane gets
focus, type, capture-view, and attach, but NOT the agent-only intelligence (digest,
1/2/3 approval, dispatch, HQ), because there is no agent turn to reason about. Guest
share scope, when present, SHALL still gate view/type on a plain pane the same way it
gates an agent pane.

#### Scenario: Typing into a plain pane

- **WHEN** a user sends input to a `plain`-tier pane from a gtmux surface
- **THEN** the input reaches that pane via `send`, and the surface does not offer the
  1/2/3 approval card or dispatch actions for it

### Requirement: General pane management lives on a separate surface

The sessions/panes browser (menu-bar/phone/web) that lists all panes SHALL be a
SEPARATE, opt-in surface, distinct from the agent radar. Plain (non-agent) panes
SHALL NOT appear on the agent radar by default. This boundary is required so the
radar keeps answering "which AGENT needs you" without being diluted into a generic
tmux session list. A surface MAY link from an agent's radar row to the browser, but
MUST NOT merge plain panes into the radar's default agent listing.

#### Scenario: Opening the browser does not change the radar

- **WHEN** a user opens the sessions/panes browser and then views the agent radar
- **THEN** the radar shows only agent panes (plus any opt-in watched panes), unchanged
  by the browser being available

### Requirement: The browser says it is reading before its first list lands

A browser that fetches its list SHALL show the brand-mark loading placeholder, with a
line naming what it is reading, from the moment it opens until the first read of the pane
list lands. It SHALL NOT show a blank list, and it SHALL NOT state an empty result or a
count of zero before that read has settled: a blank page and "no panes" are
indistinguishable to the reader, and a zero on a machine with twenty panes is a false
statement, not a placeholder. Once the read lands the placeholder SHALL leave and the rows
(or the empty statement) SHALL take its place.

#### Scenario: Opening the browser on a slow link

- **WHEN** the phone opens the pane browser and the pane list has not yet come back
- **THEN** the list area shows the loading mark and "Reading panes on <machine>", and the
  header carries no count; when the list lands the rows replace it

#### Scenario: The list comes back empty

- **WHEN** the first read lands with no panes
- **THEN** the loading mark is gone and the empty statement is shown, as before

### Requirement: The web browser keeps a failed read apart, and stays one view

The web's All panes browser SHALL tell a read of the pane list that failed from one that
came back empty: before any read has landed, a failed read SHALL say the panes on this Mac
could not be read and that it is trying again, with the count line saying it could not
read; after a read has landed, a failed refresh SHALL keep the rows and mark the count as
not refreshed. When the window crosses the width at which the web switches to its
workbench, an open All panes browser SHALL stay the one view on screen; the workbench
SHALL NOT be drawn beneath it.

#### Scenario: The web's first read fails

- **WHEN** the web opens All panes and the read of the pane list fails
- **THEN** it says the panes on this Mac could not be read and that it is trying again,
  and nothing says there are no panes

#### Scenario: Widening the window over All panes

- **WHEN** All panes is open in a narrow window and the window is widened past the
  workbench width
- **THEN** All panes is still the only view shown

### Requirement: A browser session groups fold, and says what it holds when folded

A browser SHALL group panes by session and let a user fold a group, remembering the
choice across openings, with a control to fold or unfold every group at once. It
SHALL also offer a text filter over session, window, command, title and directory.

A folded group's header SHALL still carry its rollup — its pane count, its agent
count, and a per-status count for each non-zero agent state, with waiting in the
waiting color. Folding must not be able to hide the fact that something inside is
waiting on the user; that is what the surface exists to answer.

An agent-tier row SHALL show the agent's REAL status, joined from the radar by pane
id, rather than identity alone. Surfaces SHALL agree on how a row is labelled: an
agent row SHALL NOT fall back to the raw command before the radar join (a Claude 2.x
pane's command is its version string), and a plain row whose title is only a
filesystem path SHALL fall back to the command.

#### Scenario: A folded session still reports a blocked agent

- **WHEN** a session whose group is folded contains a pane waiting on the user
- **THEN** its header shows the waiting count in the waiting color

#### Scenario: The fold survives closing the browser

- **WHEN** a user folds a session and closes the browser, then reopens it
- **THEN** that session is still folded

### Requirement: The browser opens at the size its sessions need

A desktop browser window SHALL open at a height derived from its MEASURED content —
its chrome plus the height its session list wants — bounded below so a small fleet
still gets a usable window, and above by what the display can hold. A fixed height
serves a machine with three sessions and one with eighty equally badly.

Because the pane list is fetched asynchronously, the window SHALL keep fitting for a
short interval after it is opened rather than sizing once on the first measurement,
which lands before any panes have arrived. Once a user resizes the window themselves,
the size is theirs and the app SHALL stop adjusting it.

#### Scenario: A machine with many sessions

- **WHEN** the browser is opened on a machine whose sessions need more room than the
  display can give
- **THEN** the window opens at the height the display allows and scrolls within it,
  rather than at a fixed height that shows a fraction of them

#### Scenario: The user takes over the size

- **WHEN** a user has resized the browser window themselves
- **THEN** reopening it keeps their size

### Requirement: Opt-in "watch this pane" promotion

A user SHALL be able to explicitly promote a chosen plain pane so it appears on the
radar as a distinct WATCHED row, and SHALL be able to remove it as easily. A watched
row SHALL be visually and semantically distinct from an agent row — it carries a
watched indicator, NOT an agent status (waiting/working/idle) — because those states
are agent concepts a plain pane does not have. Promotion SHALL be user-initiated only
(never automatic), and a watched pane SHALL be dropped automatically when its pane no
longer exists.

#### Scenario: Promote and auto-drop a watched pane

- **WHEN** a user watches a plain pane, then later that pane is closed
- **THEN** while it exists the pane shows on the radar as a distinct watched row (not
  an agent status), and once closed it is removed from the radar automatically

### Requirement: The pane browser shows the tmux hierarchy with stable, sigil'd ids

Every pane-browser surface (mobile, menu-bar, web) SHALL render an explicit three-level
hierarchy — session → window → pane — where each tmux level carries its stable, sigil'd id
alongside a human gloss: the session by NAME, the window as `@<window_id> <window-name>`,
and the pane as `%<pane_id> <label>`. The pane label SHALL be gtmux's derived label (the
agent name + status for agent panes, else the command / directory), NOT the raw
`pane_title`. The `%<pane_id>` SHALL be visible on every row (not only an internal key), so
two panes are told apart by a stable id rather than only by the volatile label and the
mutable `window.pane` index.

#### Scenario: Two windows in one session are distinct

- **WHEN** a session has two windows whose indices differ but whose names collide (or are
  both empty/auto-named)
- **THEN** each appears under its own `@<window_id>` sub-group, told apart by the stable
  window id, not by a name or an index that repeats across sessions

#### Scenario: Every session draws the same three levels

- **WHEN** a session holds exactly one window
- **THEN** the window band is drawn anyway, so the tree has one shape everywhere: a
  conditional band put a single-window session's panes at the indent that elsewhere means
  "window", making the same shape mean two different things one row apart. The session
  header SHALL also name the window ids it holds, so a COLLAPSED session says what is
  inside it

#### Scenario: A plain shell pane is not labeled with the host name

- **WHEN** a plain shell pane's `pane_title` is empty or the machine host name
- **THEN** the row shows `%<pane_id>` plus the command/dir gloss, never the host-name title

### Requirement: The pane id is copyable as a command

Tapping or clicking a row's `%<pane_id>` SHALL copy `gtmux focus %<pane_id>` — the id made
runnable — and SHALL confirm in place, because a copy that shows nothing cannot be told from
a tap that missed. The gesture SHALL be scoped to the id: the row itself keeps its own
action (open the pane). Every surface SHALL copy the SAME string.

#### Scenario: Copying does not open the pane

- **WHEN** the user taps the `%N` on a row
- **THEN** the command is copied and the browser stays where it is; the pane opens only
  when the row itself is tapped

#### Scenario: The web surface copies without a secure context

- **WHEN** the web surface is reached over a plain `http://<lan-ip>:8765`, where
  `navigator.clipboard` does not exist
- **THEN** the copy still lands, via the pre-Clipboard-API path

#### Scenario: The ids are searchable

- **WHEN** the user types `%23` or `@17` (or the bare digits) into the browser's search
- **THEN** the matching pane / window rows are found — the id is a search key, not only a
  label

### Requirement: A row does not repeat what its icon already says

An agent row SHALL NOT print the agent's name as a secondary line beside its official icon.
The icon carries identity; the row's text carries the work. Surfaces that can hold the name
elsewhere at no cost (a tooltip) MAY do so, for the case the icon falls back to a monogram.

#### Scenario: Six agent rows of the same agent

- **WHEN** several panes run the same agent
- **THEN** each row's text says what that pane is doing, and the agent name appears once per
  row only as its icon — not as a repeated line of prose

### Requirement: Verified HQ identity in pane browsers

The pane producer SHALL carry the radar's supervisor role on the verified HQ agent
pane as an optional `role` field. It SHALL not classify HQ independently from names
or directory paths, or give native/watched plain panes that role. Guest filtering
SHALL continue to apply per pane before rows reach a client.

Phone, iPad, menubar and Web pane browsers SHALL mark the containing session header
and the supervisor pane with a neutral HQ badge. Older-core rows MAY use the radar
join by pane id. A confirmed HQ's legacy default session name `HQ` or `hq` SHALL
be displayed as `Gtmux HQ`; custom names SHALL be retained. Grouping, collapse,
focus and locators SHALL retain raw names/ids. The display name SHALL be searchable,
and filtering a sibling pane SHALL not remove the session's verified HQ marker.
The terminal pane tree SHALL mark the verified supervisor without changing locators.

#### Scenario: Legacy HQ and an ordinary same-named session

- **WHEN** one agent has the verified supervisor role and another session is merely named HQ
- **THEN** only the verified HQ is marked; its legacy name displays as Gtmux HQ

#### Scenario: Search and fold a mixed session

- **WHEN** a verified HQ shares a session with a plain shell
- **THEN** searching the displayed HQ name finds the supervisor, searching the shell
  retains the group's HQ marker, and folding uses the original session key

### Requirement: Panes report when a terminal last showed them

`gtmux panes --json` (and `GET /api/panes`) SHALL carry an additive, optional `viewed_at`
on each pane that an attached tmux client is currently showing: the newest
`client_activity` among those clients, in unix seconds. A pane no client shows SHALL omit
it.

#### Scenario: Two terminals on two panes

- **WHEN** one terminal shows %1 and was used after another terminal showing %2
- **THEN** %1's `viewed_at` is newer than %2's, and a pane neither shows has none
