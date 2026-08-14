# pane-browser Specification

## Purpose
TBD - created by archiving change tiered-pane-control. Update Purpose after archive.
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
