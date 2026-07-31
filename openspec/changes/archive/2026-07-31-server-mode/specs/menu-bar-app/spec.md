# menu-bar-app — delta

## ADDED Requirements

### Requirement: Server mode shows a persistent, always-visible indicator

While server mode is on, the app SHALL show a dedicated, persistent menu-bar indicator —
its own status item, in the shape of a recording indicator: present for exactly as long as
the state is on, gone the moment it ends. Because server mode has no expiry and ends only
when the user ends it, this indicator is the mechanism that keeps the state from being
forgotten, so it SHALL NOT be hideable: it SHALL remain visible even under the
hide-when-idle status-item preference, which governs the agent radar's status item only.
Clicking the indicator SHALL open a small menu that states the tier, how long it has been
on, and — as its primary action — turns server mode off.

The indicator SHALL NOT alter or overlay the existing agent status item, whose glyph
continues to encode the fleet's most-urgent AGENT state only, and server mode SHALL NOT
appear as a row or section in the agent radar.

#### Scenario: On and unmissable

- **WHEN** server mode is on
- **THEN** a dedicated indicator is visible in the menu bar alongside (not on top of) the
  agent status item, and it stays visible even when the user has chosen to hide the status
  item while idle

#### Scenario: Turning it off from the indicator

- **WHEN** the user clicks the indicator and chooses to turn server mode off
- **THEN** sleep is restored and the indicator disappears

#### Scenario: Off leaves no trace

- **WHEN** server mode is not on
- **THEN** no server-mode indicator is present, and the menu bar looks exactly as it did
  before the feature existed

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
also be manageable from Preferences alongside remote access, showing the live tier, how
long it has been on, and a way to turn it off.

#### Scenario: First enable

- **WHEN** the user turns server mode on from the menu bar for the first time
- **THEN** the explainer card appears before any system dialog, states that it stays on
  until switched off, and declining it leaves every system setting unchanged
