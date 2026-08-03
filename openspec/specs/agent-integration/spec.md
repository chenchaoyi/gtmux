# agent-integration Specification

## Purpose
Define how a coding agent is integrated into gtmux: one per-agent manifest that is the single
source of truth for every subsystem, a declared support tier, an install mechanism that admits
both command-hook and plugin extension models, and a documented onboarding process whose
pitfalls checklist keeps new integrations from re-hitting known traps.
## Requirements
### Requirement: An agent's identity is defined by one manifest that is the single source of truth

Each supported coding agent SHALL be defined by exactly one manifest that is the single source
of truth for its IDENTITY and cross-cutting MEMBERSHIP: its keys and aliases, display label,
detection commands, idle glyph, icon, resume command, resource-attribution name, whether it
feeds the hook event stream, and which shared capabilities it has (transcript, headless). Each
subsystem that consumes these SHALL derive its list from the registry rather than keep its own
agent-keyed copy; adding an agent's identity SHALL require authoring one manifest and
registering it, with no edit to those subsystems.

DOMAIN-specific behavior — the classifier's event-semantics tables, the prompt/ready
signatures, and the hook-install artifact — MAY remain in its own package (keyed by the
manifest's agent key), because relocating a domain type into the pure identity registry is
over-abstraction. Where it does, a conformance check SHALL bind it back to the registry so it
cannot silently drift or be forgotten (e.g. every hook-equipped agent MUST have an install
path), and the onboarding playbook SHALL enumerate these touchpoints.

#### Scenario: A new agent's identity is added in one place

- **WHEN** a coding agent is added by authoring and registering one manifest
- **THEN** it is detected by the radar, resumable, resource-attributed, and — if hook-equipped
  — known to the driver, with no change to the driver, radar, hook, resume, or resource
  subsystems themselves

#### Scenario: A hook-equipped agent cannot ship dark

- **WHEN** the conformance check runs
- **THEN** it fails if an agent the registry marks hook-equipped has no install path (its
  event layer would otherwise stay dark, as opencode's did before it had an installer)

### Requirement: A manifest declares its support tier and degrades gracefully

A manifest SHALL declare its support tier: **Tier 1** provides an install spec so the agent
emits the events that drive waiting/done detection, receipt-backed dispatch verification, and
notifications; **Tier 2** additionally provides a transcript parser so the digest can render
goal/last/ask. A manifest MAY declare Tier 1 only. Any capability a manifest does not provide
SHALL degrade to the existing screen-read fallback rather than fail — the absence of a
transcript parser or of a hook event SHALL never regress an agent below what a manifest-less
agent already gets.

#### Scenario: A Tier 1 agent has no digest parser

- **WHEN** an agent declares Tier 1 (events) but no transcript parser
- **THEN** its radar, waiting/done, and receipt-verified dispatch work, and the digest falls
  back to its screen-derived form without error

#### Scenario: A hook event never arrives

- **WHEN** a hook-equipped agent's session does not emit an expected event
- **THEN** verification falls to the two-frame screen read, exactly as for an agent with no
  hook (the absence of evidence is not a failure)

### Requirement: The install spec supports both command-hook and plugin extension models

The manifest's hook-install spec SHALL support materializing the integration by either a JSON
"run a command on event" configuration OR a plugin artifact (e.g. a JavaScript module) that
subscribes to the agent's native events and shells out to `gtmux hook`. An agent whose only
extension point is a plugin system, not a command-hook file, SHALL be installable to full Tier
1 parity through the same `install-hooks --agent <key>` entry point and removed cleanly on
uninstall.

#### Scenario: A plugin-only agent is wired to Tier 1

- **WHEN** `install-hooks --agent <key>` runs for an agent whose extension model is plugins
- **THEN** gtmux writes a plugin that forwards the agent's lifecycle events to `gtmux hook`,
  and the agent thereafter drives waiting/done, receipt, and notifications like a
  command-hook agent

#### Scenario: Uninstall removes only what gtmux wrote

- **WHEN** the agent's integration is uninstalled
- **THEN** the gtmux-written hooks or plugin are removed and any pre-existing user
  configuration for that agent is left intact

### Requirement: Agent identity is resolved from the process subtree

The identity of the agent running in a pane SHALL be resolved by inspecting the pane's process
subtree, not the pane's foreground command, because an agent frequently does not present its
own name there (it may report its version, or run under a bare runtime such as `node`). Every
per-agent decision keyed on identity — including whether a pane is hook-equipped — SHALL use
this resolution.

#### Scenario: A renamed-process agent is identified

- **WHEN** an agent's foreground command is its version string or a bare runtime name rather
  than its own command
- **THEN** the agent is still identified from its process subtree and treated as its manifest
  declares (detected, hook-equipped, dispatched)

### Requirement: Onboarding is a documented, checklisted process

The documentation SHALL carry an agent-onboarding playbook: a step-by-step process for adding
an agent and a pitfalls checklist of the failure modes previous integrations paid for
(identity via subtree not foreground command; a hook that must be installed or the event layer
stays dark; plugin vs command-hook extension models; locale/glyph loss over daemon-spawned
PTYs; sparse events falling back to screen verification; idle-glyph classification requiring
live-process confirmation; no third-party trademarks in icons). The playbook SHALL be
referenced from the repository's contributor guide.

#### Scenario: A contributor follows the playbook to add an agent

- **WHEN** a contributor opens the onboarding playbook to add a new agent
- **THEN** it lists every manifest field to fill, the order to verify them, and the pitfalls
  checklist to check against before the integration is considered done

