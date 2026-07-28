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

