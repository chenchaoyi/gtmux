# session-restore (delta)

## ADDED Requirements

### Requirement: A conversation resumes the way it was started

An agent is not always launched by its own name. A user's environment may put a wrapper
in front of it — one that supplies configuration, credentials or a network the plain
binary does not have — and a resume that ignores this brings the conversation back as a
different thing, or not at all.

The system SHALL record how an agent was launched, alongside which agent it is and where
it was running, and SHALL reproduce that command when resuming the conversation. Where no
launch command was recorded, it SHALL fall back to the agent's generic resume form, so a
record written before this existed resumes exactly as well as it did.

Recording a command SHALL NOT widen what is resumable: the decision about whether a saved
pane ran an agent is unchanged, and a recorded command is replayed only for a pane that
decision already accepts.

#### Scenario: An agent behind a wrapper

- **WHEN** a session was started through a wrapper and is later resumed
- **THEN** it is resumed through that same wrapper

#### Scenario: A record from before this existed

- **WHEN** a resume record carries no launch command
- **THEN** the agent's generic resume form is used, as before

#### Scenario: A pane that ran no agent

- **WHEN** the saved layout shows a pane was a plain shell
- **THEN** nothing is resumed into it, whatever any record remembers
