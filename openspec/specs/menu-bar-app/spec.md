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
SHALL exclude supervisor rows). The card carries the gtmux brand pane-grid mark
as its avatar (the supervisor is gtmux's own concept — visually distinct from
agent avatars), the standard status badge, and the session's task line; clicking
it focuses the supervisor's pane. When no supervisor is live, the slot SHALL
show a quiet "not running — start" affordance that launches `gtmux hq` (the app
stays a CLI consumer).

#### Scenario: Supervisor live

- **WHEN** an `agents --json` row carries `role:"supervisor"`
- **THEN** the popover shows the HQ card (brand mark + status badge + task) above
  the sections, and that row does NOT appear inside any section

#### Scenario: Supervisor absent

- **WHEN** no row carries `role:"supervisor"`
- **THEN** the HQ slot shows the quiet start affordance, and clicking it shells
  `gtmux hq`

### Requirement: Shared-input control surface

The menu-bar app SHALL provide a host control surface for web-shared input that
mirrors `gtmux share`, so the host can consent to and scope guest typing without
dropping to a terminal. The controls SHALL live in a "Shared input" section of
Preferences, beside Remote access (guests arrive over the same serve/tunnel):

- a **consent toggle** (default reflecting the current state; OFF by default),
  which turns shared input on/off;
- a **per-pane allowlist** rendered from the live agent list — each tmux pane
  (`source == "tmux"`, a real `%N`) a checkbox the host ticks to allow guest input
  into that pane. Each row SHALL carry the SAME identity the session list shows —
  the agent avatar (official icon + state), the agent's own session title
  (`primary`), and a dim `session · %pane` line — and be ordered like the radar
  (state rank → session title), so the host ticks the pane they RECOGNISE from the
  popover, never an indistinguishable generic agent name repeated down the list;
- **guest share links**: existing links listed with a per-link revoke, and a
  "new share link" action that mints a link and copies its URL to the clipboard.

The app SHALL remain a pure CLI consumer: it MAY read the local `share.json` for
the consent/allowlist state, but SHALL perform every mutation by invoking
`gtmux share …`, and SHALL obtain the guest list and minted URL from the CLI's
token-free `--json` output (never by reading the token roster). The server gate
stays authoritative; the app only reflects and drives it.

When shared input is LIVE (consent on AND at least one allowed pane AND at least
one guest link), the popover SHALL show a quiet exposure indicator — a
type-into-terminal exposure is never silent, the same ethos as the "Remote on"
indicator.

#### Scenario: Host consents and allows a pane from the menu bar

- **WHEN** the host turns the Shared-input toggle on and ticks a tmux pane in the allowlist
- **THEN** the app invokes `gtmux share on` and `gtmux share add %N`, and the section reflects the new state (that pane is now guest-typable while consent is on)

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

- **WHEN** consent is on, at least one pane is allowed, and at least one guest link exists
- **THEN** the popover shows a compact shared-input exposure indicator that opens Preferences when tapped

