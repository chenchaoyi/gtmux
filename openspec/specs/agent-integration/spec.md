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
- **THEN** each supplied identity field feeds its consumer: detection commands feed radar,
  resume argv enable resume, a resource name enables attribution, and hook-equipped membership
  feeds the driver, without editing those consumers' agent lists

#### Scenario: A hook-equipped agent cannot ship dark

- **WHEN** the conformance check runs
- **THEN** it fails if an agent the registry marks hook-equipped has no install path (its
  event layer would otherwise stay dark, as opencode's did before it had an installer)

### Requirement: A manifest declares its support tier and degrades gracefully

A manifest SHALL declare its capabilities: **Tier 1** provides an install spec so the agent
emits the events that drive waiting/done detection, receipt-backed dispatch verification, and
notifications; **Tier 2** additionally provides a transcript parser for the digest's
`goal`/`last`. `ask` SHALL remain a separate read of a waiting pane's numbered options.
A manifest MAY declare Tier 1 only. Absent receipt/readiness evidence SHALL fall back to
screen checks. A missing or disabled content reader SHALL leave `goal`/`last` absent,
without disabling radar or input. A missing or disabled headless capability SHALL refuse
`spawn --oneshot`, as specified by `agent-driver`, rather than start an interactive agent.
Usage SHALL remain independent of the content switch, but still requires a supported log.

#### Scenario: A Tier 1 agent has no digest parser

- **WHEN** an agent declares Tier 1 (events) but no transcript parser
- **THEN** its radar, waiting/done, and receipt-backed dispatch remain available; digest
  `goal`/`last` are absent, while pane-derived fields remain available without a transcript

#### Scenario: A hook event never arrives

- **WHEN** a hook-equipped agent's session does not emit an expected event
- **THEN** verification falls to the two-frame screen read, exactly as for an agent with no
  hook (the absence of evidence is not a failure)

### Requirement: Codex turn completion is attributed to its own session and pane

When a Codex Stop hook lacks session and cwd identity, gtmux SHALL NOT trust an inherited
`TMUX_PANE` as proof of ownership. It SHALL attribute the Stop only if one live pane has
an active session binding whose own rollout has just completed. An active turn marker
without a session ID MAY be claimed only when that bound rollout completed after the
marker was created. If the hook cannot be
attributed, the radar SHALL reconcile that pane's active and waiting state from a later
`task_complete` in its bound session rollout. A completion before the current turn's
markers, or superseded by a later `task_started`, SHALL NOT end the current turn.

#### Scenario: Shared app-server sends a pane-less Codex Stop

- **WHEN** a Codex Stop arrives with another pane's inherited `TMUX_PANE` and no session
  or cwd, while exactly one bound active Codex session just logged `task_complete`
- **THEN** only that session's pane is marked done; the inherited pane is not changed

#### Scenario: Codex completion hook cannot be attributed

- **WHEN** a Codex pane still has a waiting marker but its bound active session has logged
  `task_complete` after that marker
- **THEN** the radar clears the stale wait and reports that pane idle

#### Scenario: Codex prompt omitted its session ID

- **WHEN** an active Codex marker contains no session ID and its pane has a bound
  conversation whose latest rollout event is `task_complete` after that marker
- **THEN** the Stop hook or the next radar read marks only that pane idle
- **AND** a completion before the marker or a marker naming another session does not end it

#### Scenario: An idle Codex pane repaints a usage warning

- **WHEN** a bound Codex pane has no active or waiting turn marker, its own latest rollout boundary is `task_complete`, and an idle warning or other TUI repaint changes its screen
- **THEN** the radar SHALL keep it idle instead of reporting a new working turn or a later false completion
- **AND** a later `task_started`, a current turn marker, or an unbound pane SHALL continue to use the normal working signals

### Requirement: The install spec supports command-hook, plugin, and managed-block extension models

The manifest's hook-install spec SHALL support materializing the integration by a JSON
"run a command on event" configuration, a plugin artifact (e.g. a JavaScript module) that
subscribes to the agent's native events and shells out to `gtmux hook`, OR a delimited
block appended to a configuration file the USER owns. An agent whose only extension point
is a plugin system, or whose hooks share a file with the user's own settings, SHALL be
installable to full Tier 1 parity through the same `gtmux install hooks --agent <key>` entry
point and removed cleanly on uninstall.

#### Scenario: A plugin-only agent is wired to Tier 1

- **WHEN** `gtmux install hooks --agent <key>` runs for an agent whose extension model is plugins
- **THEN** gtmux writes a plugin that forwards the agent's lifecycle events to `gtmux hook`,
  and the agent thereafter drives waiting/done, receipt, and notifications like a
  command-hook agent

#### Scenario: An agent whose hooks live in the user's own config file

- **WHEN** `gtmux install hooks --agent <key>` runs for an agent whose hooks are entries in a
  configuration file that also holds the user's own settings
- **THEN** gtmux appends its entries as a single delimited block and rewrites nothing else
  in that file, and re-running the install replaces that block rather than adding a second

#### Scenario: Uninstall removes only what gtmux wrote

- **WHEN** the agent's integration is uninstalled
- **THEN** the gtmux-written hooks, plugin, or block are removed and any pre-existing user
  configuration for that agent is left intact, byte for byte

### Requirement: A logless agent reaches Tier 2 via a gtmux-owned transcript

An agent that persists no readable conversation log on disk SHALL still be able to reach Tier 2
(digest goal/last) by having gtmux keep the transcript itself: the agent's plugin streams
the user prompt and the final assistant text through `gtmux hook` alongside the agent's session
id, and gtmux appends them to its own per-session store that the transcript parser reads.
Setting the manifest's transcript-parser key SHALL auto-wire the digest content channel, and the
session id the plugin pipes SHALL be the key that lines the store up with the resume record the
digest resolves.

#### Scenario: opencode's digest renders goal/last with no on-disk log

- **WHEN** an opencode session runs a turn with the gtmux plugin installed
- **THEN** gtmux records the user prompt and the final assistant reply to its own transcript
  store, keyed by the session id the plugin piped, and the digest renders that turn's goal/last
  with perception tier `driver`

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
live-process confirmation; official identity icons with recorded sources and neutral fallback marks). The playbook SHALL be
referenced from the repository's contributor guide.

#### Scenario: A contributor follows the playbook to add an agent

- **WHEN** a contributor opens the onboarding playbook to add a new agent
- **THEN** it lists every manifest field to fill, the order to verify them, and the pitfalls
  checklist to check against before the integration is considered done

### Requirement: Agent capability declarations are wired to implementations

The registry conformance checks SHALL verify that every declared transcript capability has a
registered parser, every hook-equipped agent has a hook installer and display mapping, and
every agent declaring dedicated event semantics has a classifier table. A missing capability
implementation SHALL fail the check with the agent key and capability named.

#### Scenario: A declared transcript parser is missing

- **WHEN** an agent manifest declares transcript support without a parser
- **THEN** the conformance check fails and names that agent and transcript capability

### Requirement: The Codex transcript parser declares its observed event shapes

The Codex Tier 2 transcript parser SHALL have sanitized fixtures for the event shapes it
claims to support. Unknown event records SHALL be skipped without turning injected context
into a user prompt or preventing later recognized turns from being read.

#### Scenario: A transcript contains an unknown event

- **WHEN** the Codex parser reads an unknown event between recognized records
- **THEN** it ignores that record and continues parsing subsequent recognized turns
