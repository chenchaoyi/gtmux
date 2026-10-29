## ADDED Requirements

### Requirement: Native session trace tools

The native App SHALL expose Trace through a session-tools control, alongside Diff
when available. Chat and Terminal SHALL remain presentation modes. Trace SHALL share
the parent transcript reader without replacing its composer or restarting an active
Chat subscription. HQ and readable external sessions SHALL use the same reader UI;
plain shells SHALL NOT offer Trace. External sessions SHALL remain read-only.

#### Scenario: Inspect without changing mode
- **WHEN** a user opens Trace from Chat and closes it
- **THEN** the mounted conversation, unsent draft and reading position remain
- **AND** opening or closing the tool does not start a duplicate transcript read

### Requirement: Adaptive trace reading

Trace SHALL provide a virtualized list, search over loaded history, type and session
filters, explicit follow, original-text Copy, Markdown/source and recorded input,
working directory and truncation notices. Details SHALL be closable. Compact views
SHALL preserve the mounted list and return to its selected record; wide views SHALL
use measured content width and font scale to split list/details. Closing details
SHALL expand the list. Absent times and outcomes SHALL NOT be fabricated.

#### Scenario: Freeze a selected revision
- **WHEN** a selected record changes during streaming or history is prepended
- **THEN** its displayed text and historical speaker remain together
- **AND** an explicit update action adopts the latest recorded version

#### Scenario: Retire private records
- **WHEN** client, credential, pane, home or session identity changes
- **THEN** old records, selections and profiles cannot appear in the new scope
- **AND** a delayed old read cannot replace its history

#### Scenario: External read access is revoked
- **WHEN** the external conversation returns HTTP 401 or 403
- **THEN** the cached conversation clears and Trace closes

### Requirement: Shared trace indexing

The browser and native readers SHALL consume one typed pure event model through a
generated browser asset and shared synthetic fixtures. A generation check SHALL
reject stale or manually changed output. Historical profiles and files SHALL use
the selected message identity through the existing authorized readers.

#### Scenario: Compare providers
- **WHEN** Codex, Claude, Kimi, OpenCode or DSH records are indexed by either reader
- **THEN** the same fixture yields the same ordered records, inputs and session boundaries
