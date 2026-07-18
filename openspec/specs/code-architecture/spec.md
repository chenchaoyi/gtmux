# code-architecture Specification

## Purpose
Keep gtmux's Go core decomposed into packages with compiler-enforced, one-way import
boundaries — the agent-radar kernel, the HQ supervisor subsystem, the dispatch bridge, and
the top-level command layer — so an accidental dependency cycle is a build error rather
than a code-review note, and the pane-data kernel stays testable in isolation. Realized by
the `decompose-app-package` change (`internal/radar`, `internal/hq`, `internal/dispatchbridge`,
`internal/panefocus` under `internal/app`).

## Requirements
### Requirement: The radar kernel and HQ subsystem are acyclic, one-way-layered packages

The system SHALL keep the agent-radar kernel (pane sensing / classification / digest
production) and the HQ supervisor subsystem as packages with a STRICT one-way import
layering, so an accidental dependency cycle is a COMPILE error rather than a code-review
note. Specifically: the radar kernel SHALL import only leaf packages (tmux, dispatch,
prompt, resume, state, native, transcript, i18n) and SHALL NOT import the HQ subsystem or
the top-level command package; the HQ subsystem MAY import the radar kernel and the
dispatch bridge but SHALL NOT import the top-level command package; and nothing SHALL
import the top-level command package (it is the top of the graph). The dispatch bridge
(the tmux/events adapter shared by the command layer and the HQ subsystem) SHALL be a
leaf so both can use it without a cycle.

#### Scenario: The radar kernel does not depend on the supervisor or the command layer

- **WHEN** the radar package is compiled
- **THEN** it imports no HQ/supervisor package and no top-level command package — only
  leaf packages — so the pane-sensing kernel can be tested and reused without pulling in
  the supervisor or CLI

#### Scenario: The HQ subsystem does not depend on the command layer

- **WHEN** the HQ package is compiled
- **THEN** it imports the radar kernel and the dispatch bridge but NOT the top-level
  command package, so `hq → app` can never form a cycle

#### Scenario: A layering violation fails the build

- **WHEN** an edit makes the radar kernel import the HQ subsystem (or either import the
  command package)
- **THEN** the module no longer compiles — the boundary is enforced by the compiler, not
  by convention

