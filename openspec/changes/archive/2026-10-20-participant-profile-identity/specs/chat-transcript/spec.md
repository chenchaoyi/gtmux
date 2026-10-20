## ADDED Requirements

### Requirement: Compact profiles preserve identity scope

The participant dialog SHALL fit settled content within its viewport limit, retain
scrolling for longer details and show a stable labelled placeholder while loading.
Known provider identifiers SHALL use canonical brand casing in display only;
unknown/custom identifiers and raw metadata SHALL remain unchanged. An HQ profile
SHALL explain its coordinating role. A message sender's recorded agent and the
current supervisor's agent SHALL be explicitly distinct. Current HQ identity SHALL
come only from an unambiguous supervisor row on the same connected server;
missing historical identity SHALL NOT be inferred from the recipient or current HQ.
The global reply-fold control SHALL display brand-coloured localized Collapse all
or Expand all text, separate from per-reply and tool disclosures.

#### Scenario: HQ sender has no historical agent identity

- **WHEN** an HQ sender profile has no recorded agent but the connected server has one supervisor
- **THEN** the profile explains HQ's role and labels that supervisor's agent as current only, without claiming the message-time agent or borrowing the recipient's facts

#### Scenario: Custom provider identifier

- **WHEN** a provider fact identifies a custom gateway rather than a recognized provider
- **THEN** its identifier is displayed unchanged and its raw fact remains intact
