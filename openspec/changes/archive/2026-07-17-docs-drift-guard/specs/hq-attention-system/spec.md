# hq-attention-system — delta

## MODIFIED Requirements

### Requirement: Split feeding-HQ from showing-user

The system SHALL feed the supervisor the FULL event stream through a channel that is
NOT visible in the HQ pane, so that the only user-visible action is HQ's deliberate
print. gtmux SHALL NOT force-type low-value (`routine`/QUIET) event lines into the HQ
pane as a way to inform HQ. HQ's awareness of an event SHALL be independent of whether
the user is shown anything about it.

#### Scenario: Low-value events reach HQ silently

- **WHEN** a QUIET-tier event (e.g. a resolved wait, a send-landed confirmation, a
  working tick) occurs and an HQ pane is live
- **THEN** HQ receives it through the silent feed and gtmux does NOT type a visible
  `» gtmux·<class>` wake line for it into the HQ pane

#### Scenario: HQ omniscience is decoupled from user surfacing

- **WHEN** any event occurs while an HQ pane is live
- **THEN** the event is delivered to HQ regardless of surfacing tier, and only a
  CRITICAL/NORMAL judgment by HQ produces user-visible output
