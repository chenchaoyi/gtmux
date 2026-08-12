# supervisor-agent (delta)

## ADDED Requirements

### Requirement: The HQ verdict is decided once, in the core

The system SHALL resolve the supervisor's overall verdict in the CORE and expose it on the
digest, so every surface reads one judgment rather than re-deriving its own.

The verdict SHALL be a priority-ordered state: the supervisor is ABSENT; else the
supervisor itself is waiting on the user; else one or more workers are waiting; else the
machine is at its critical resource tier; else the supervisor is working; else normal. The
ordering is part of the contract — a surface SHALL NOT re-order it.

The core SHALL NOT serve a rendered headline sentence. The headline is user-facing prose
and each surface owns its own language state, so a pre-rendered string would override a
reader's language choice. The core SHALL instead serve the FACTS a headline is built
from — how many workers are waiting, who has waited longest, how many others are normal,
how many worker sessions exist — and each surface SHALL render its own sentence from the
state and those facts.

The object SHALL be additive and OPTIONAL, so a surface built against an older core keeps
working; a surface SHALL keep a local fallback for when it is absent.

#### Scenario: A red resource tier is never reported as normal

- **WHEN** the machine is at its critical resource tier and no agent is waiting
- **THEN** the served verdict is the resource state, and no surface can render "all
  normal" beside a red token — the contradiction that existed while each surface decided
  for itself

#### Scenario: The supervisor's own call outranks a quiet fleet

- **WHEN** the supervisor itself is waiting on the user while every worker is idle
- **THEN** the served verdict says the supervisor needs a decision, on every surface

#### Scenario: An older core is tolerated

- **WHEN** a surface receives a digest with no verdict object
- **THEN** it falls back to its local resolver rather than showing nothing
