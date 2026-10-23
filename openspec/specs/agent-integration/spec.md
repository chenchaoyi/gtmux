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

#### Scenario: A delimited block whose bounds are unclear is refused

- **WHEN** install or uninstall finds the block's sentinels unpaired in the user's file (an
  opening one with no closing one, a closing one with no opening, or one opened inside another)
- **THEN** gtmux writes nothing, leaves the file and any earlier backup as they were, and says
  which line is unclear and how to fix it, rather than guessing where the block ends

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

### Requirement: Hook terminal outcomes preserve interruption and failure semantics

The system SHALL distinguish successful completion, user interruption and terminal failure using each agent's documented lifecycle payload. Interrupted or failed turns SHALL clear working and waiting without earning a completion notification or latest-completed pane update. Only a subsequent actual prompt or turn start SHALL reset the terminal outcome. Tool failures, recoverable errors and child-session endings SHALL NOT terminate the parent turn. Known mismatched turn identifiers SHALL NOT end a newer turn. Missing upstream events SHALL retain existing screen/transcript fallback rather than be invented.

#### Scenario: Codex interruption with a missing hook

- **WHEN** a pane's bound Codex rollout ends with turn_aborted after its active and waiting markers
- **THEN** its state becomes idle with an interrupted outcome and no successful-finish stamp
- **AND** a newer task_started, session mismatch or changed cwd prevents old reconciliation

#### Scenario: Late callback after cancellation

- **WHEN** a tool or permission callback reaches an interrupted tmux or native session before a real new turn starts
- **THEN** it cannot restore working or waiting or relabel that outcome as success

#### Scenario: Payload-specific and child outcomes

- **WHEN** Cursor stop reports aborted, Copilot reports a recoverable error, or OpenCode emits a child session ending
- **THEN** Cursor is interrupted, the recoverable error does not end the turn, and the child cannot change the root pane
- **AND** a subsequent OpenCode idle after root session.error does not emit successful completion

### Requirement: Hook artifacts follow current official formats and preserve compatibility

The system SHALL document the eight existing hook contracts and their limits in English and Chinese, including primary sources. Installers SHALL add supported state events, remove only gtmux's retired bindings, and preserve foreign hooks. Kiro SHALL support both CLI 2.x profile hooks and V3 global hooks, preserving foreign array-form profile hooks during migration. OpenCode SHALL install in the current global plugins directory and migrate only its owned legacy plugin. Observational hooks SHALL NOT make approval decisions; a tool-start callback that also runs on authorized tools SHALL NOT alone prove a permission wait.

#### Scenario: Current Kiro and OpenCode configuration migration

- **WHEN** gtmux upgrades to Kiro V3 globals or the current OpenCode plugin directory
- **THEN** it backs up and removes only its own duplicate legacy artifact or bindings
- **AND** foreign profile hooks, plugin contents and configuration remain preserved

### Requirement: DeepSeek Harness external conversation support

The system SHALL discover and read root user conversations from the configured DSH home using a dedicated bounded v4 JSONL/Zstandard adapter. It SHALL expose public append history through the shared CLI, server, menu-bar, phone, iPad and Web readers with the official DeepSeek mark and neutral fallback. It SHALL exclude injected context, reasoning, replay metadata and raw tool results, report omitted turns, and refuse unknown generations, corrupt committed records, ambiguous identities or encodings. Readers SHALL NOT mutate DSH artifacts, acquire writer leases, invoke a model or infer live frontend ownership from saved metadata. Registry external-reader declarations SHALL have conformance-checked adapters. TUI control, adoption and usage figures SHALL NOT be implied.

#### Scenario: Existing DSH Web is already running

- **WHEN** a root user conversation exists in the configured DSH home
- **THEN** it is discoverable and readable without restarting the Web server or installing hooks
- **AND** discovery alone does not enroll saved history in the live radar; its current controlling client remains unconfirmed and it has no pane input controls

#### Scenario: Scope and public content boundaries

- **WHEN** a reader receives a foreign home, a same-ID copy or an unsupported highest generation
- **THEN** it refuses the read rather than selecting another artifact
- **AND** a guest cannot invoke an external reader or policy mutation

### Requirement: DeepSeek Harness native events preserve explicit consent

The system SHALL provide an owned Cordis overlay and native plugin using DSH session and question APIs rather than Codex compatibility hooks. It SHALL observe only root live events, preserve successful/interrupted/failed outcomes, delegate questions unchanged, avoid treating automatic approval requests as human waits, use bounded subprocess execution and never claim its Web launcher's tmux pane. Installing SHALL preserve other profiles/plugins and SHALL NOT restart running DSH. HQ, notification and knowledge grants SHALL remain separate, revision-checked and agent/home/session scoped. Knowledge SHALL include only activity after its current grant boundary; revocation SHALL stop collection. Metadata polling SHALL NOT replay old events into HQ.

#### Scenario: Native plugin is installed for a later launch

- **WHEN** the owner installs DSH hooks
- **THEN** only two owned files are written and the next-launch overlay command is printed
- **AND** lifecycle callbacks use the dedicated native plugin without changing approval decisions

#### Scenario: Consent is revoked or a data home changes

- **WHEN** HQ follow is disabled or the same ID is copied to another home
- **THEN** dependent permissions are disabled or absent respectively
- **AND** old activity cannot become new knowledge after a later grant

### Requirement: Share provider-owned plugin writers across setup paths

Explicit hook installation and doctor repair SHALL use the same provider-declared plugin writer. A DSH write SHALL manage only the bundled native Cordis module and its exact launch overlay, honoring DSH_HOME and preserving user profiles, foreign files and unrelated OpenCode migration artifacts. Installation discovery MAY use registry-declared executable probes without enabling a TUI radar profile or resume capability.

#### Scenario: Doctor installs a fresh DSH integration

- **WHEN** DSH is detected and its native plugin files are missing
- **THEN** doctor repair writes both module and overlay through the DSH writer and leaves an unrelated OpenCode bridge untouched

#### Scenario: An owned overlay is stale or foreign

- **WHEN** the launch overlay differs from the current generated URI and configuration
- **THEN** doctor reports repair needed and repairs only recognized gtmux-owned files
- **AND** an unrecognized file causes refusal before changing either artifact

### Requirement: Explicit automatic hook installation selection

Hook installation SHALL retain its existing Claude default when no agent is named. `--agent auto` SHALL select supported detected homes/installations, preserve settings that disable hooks, and run each provider's explicit installer. A failed provider SHALL make the overall operation fail while other selected providers remain independently eligible; no supported detections SHALL create no configuration. Help SHALL list DSH and automatic selection.

#### Scenario: A detected agent has disabled hooks

- **WHEN** automatic hook installation encounters that agent
- **THEN** it skips that configuration without enabling hooks and continues other detected integrations

#### Scenario: DSH is installed without a default data directory

- **WHEN** the DSH executable is discoverable and its selected data home does not yet exist
- **THEN** setup can detect it without declaring TUI radar or resume support

### Requirement: DeepSeek Harness terminal clients preserve one harness identity

Supported terminal clients SHALL use the existing `dsh` agent identity and official
DeepSeek branding. Client-specific discovery and input behavior SHALL belong to a
modular adapter. A verified interactive tmux attachment SHALL be a managed DSH pane;
Web conversations SHALL retain their external-history behavior. A process name,
working directory or mounted-session set alone SHALL NOT authorize input.

#### Scenario: A supported terminal client controls a DSH conversation
- **WHEN** its live process, canonical home, current session and tmux attachment are verified
- **THEN** the core exposes one managed DeepSeek Harness conversation across consumers
- **AND** a second external row for the same conversation is suppressed

#### Scenario: A client uses another backend or loses its attachment
- **WHEN** the backend or foreground binding is unavailable, changed or ambiguous
- **THEN** DSH input capability is withheld or revoked without guessing a conversation
- **AND** external saved-history reading remains governed by its existing policy

### Requirement: Terminal input is addressed and acknowledged by the receiving native agent

Submitted DSH text SHALL retain the existing draft and permission safeguards and use
a bounded, one-use native request carrying the expected conversation, home and live
client instance. The receiving command SHALL validate its actual target before
queuing user input. An acceptance receipt SHALL distinguish accepted input from
execution and completion. Retries SHALL NOT create duplicate input, including when
a response is lost. A stale session, foreign request, missing bridge or revoked
client capability SHALL NOT fall back to untargeted text or another writer.

#### Scenario: A session switches during delivery
- **WHEN** the receiving native command targets a different session or client instance
- **THEN** it refuses the request before queuing its text
- **AND** it reports a refusal rather than a successful delivery

### Requirement: HQ message coverage is recoverable and review is explicit

Verified terminal conversations SHALL provide stable public-message identities and
source sequence cursors for user text and assembled assistant text. Private reasoning,
injections, raw tool results and model-surface replacement copies SHALL be excluded.
Live events MAY trigger discovery, but persisted artifacts SHALL reconcile missing
evidence incrementally with bounded memory. Missing or invalid source history SHALL
be visible. Reading or notifying SHALL NOT by itself mark a message reviewed.

#### Scenario: Live event delivery is missed
- **WHEN** a committed public message is absent from the live feed
- **THEN** the next bounded reconciliation recovers it without duplicating prior evidence
- **AND** interruption and failure retain their non-success meaning

### Requirement: Terminal-client setup verifies ownership and runtime capability

Installation and repair SHALL share the provider's writer, preserve foreign files
and configuration and declare only required native component capabilities. Doctor
SHALL distinguish installed configuration from a loaded, verified terminal bridge.
An existing agent process SHALL NOT be restarted merely to make diagnosis healthy.

#### Scenario: A bridge is installed but not loaded
- **WHEN** its owned artifacts are current but no valid runtime attachment exists
- **THEN** diagnosis reports the loading boundary and does not claim control is ready


### Requirement: Owned DSH bridges can be maintained without enabling an integration

`install refresh-owned --agent dsh` SHALL act only on an already installed recognized bridge and emit a structured result. Update completion and a new Mac app version MAY invoke this bounded maintenance before read-only checks. It SHALL preflight all artifacts, refuse foreign files and links, preserve the installed executable, publish complete files atomically and verify the result. It SHALL NOT modify user profiles, install clients, restart sessions or downgrade a newer bridge. Metadata-only gtmux version drift SHALL NOT make executable-identical bridge code stale. First installation SHALL remain explicit.

#### Scenario: A bridge is absent

- **WHEN** automatic maintenance finds no installed module
- **THEN** it reports not-installed and creates no integration files

#### Scenario: A prior release left an owned stale component

- **WHEN** all target files are recognized or absent inside the owned bridge
- **THEN** maintenance updates changed artifacts and verifies them, while preserving profiles and running sessions

### Requirement: Saved dst outer launches retain their verified harness identity

A saved direct dst or Node-hosted bin/dst launch MAY provide client liveness evidence only with a verified resumable DSH/home/client/session record. Explicit other backends, idle shells and unavailable conversations SHALL NOT gain restoration. Live input SHALL still require the original native-attachment checks. GUI command discovery SHALL preserve PATH precedence and find installed NVM versions without sourcing shell initialization; dst launch SHALL include its paired Node runtime when present.

#### Scenario: Resurrect saved the outer Node CLI

- **WHEN** the save records node /fixture/bin/dst and the locator has a verified DSH terminal resume record
- **THEN** the restore plan lists that DSH conversation and the launch targets its exact session through dst
