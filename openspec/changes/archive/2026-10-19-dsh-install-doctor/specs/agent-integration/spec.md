## ADDED Requirements

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
