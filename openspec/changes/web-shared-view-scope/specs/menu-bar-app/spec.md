## MODIFIED Requirements

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

#### Scenario: Host allows a pane to be seen, then typed into

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
