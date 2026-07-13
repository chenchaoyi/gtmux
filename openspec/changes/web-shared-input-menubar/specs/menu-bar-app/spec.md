## ADDED Requirements

### Requirement: Shared-input control surface

The menu-bar app SHALL provide a host control surface for web-shared input that
mirrors `gtmux share`, so the host can consent to and scope guest typing without
dropping to a terminal. The controls SHALL live in a "Shared input" section of
Preferences, beside Remote access (guests arrive over the same serve/tunnel):

- a **consent toggle** (default reflecting the current state; OFF by default),
  which turns shared input on/off;
- a **per-pane allowlist** rendered from the live agent list — each tmux pane
  (`source == "tmux"`, a real `%N`) a checkbox the host ticks to allow guest input
  into that pane, so panes are chosen by the agent the host recognizes;
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

#### Scenario: Minting a share link copies it

- **WHEN** the host taps "new share link"
- **THEN** the app invokes `gtmux share new --json`, shows the resulting URL, and copies it to the clipboard for the host to send to a collaborator

#### Scenario: Revoking a link from the menu bar

- **WHEN** the host taps revoke on a listed guest link
- **THEN** the app invokes `gtmux share revoke <id>`, exactly that link stops working, and it disappears from the list

#### Scenario: Live shared input is not silent

- **WHEN** consent is on, at least one pane is allowed, and at least one guest link exists
- **THEN** the popover shows a compact shared-input exposure indicator that opens Preferences when tapped
