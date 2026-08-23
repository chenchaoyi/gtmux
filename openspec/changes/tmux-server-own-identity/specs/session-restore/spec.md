# session-restore (delta)

## ADDED Requirements

### Requirement: The tmux server restore creates answers for itself

macOS attributes a file access to the process tree's root application, fixed at creation
and unchanged thereafter. A tmux server created as a child of the menu-bar app therefore
lends that app's name to everything inside it — every agent, and every command those
agents run — for as long as the server lives.

The system SHALL create the boot tmux server with an identity of its own rather than the
caller's, so that work done by a user's agents is attributed to the agents' own process
tree and not to gtmux.

This SHALL apply only where the attribution is actually wrong. A restore started from a
terminal is already attributed to that terminal, which is the application the user
launched and therefore the honest answer; only a caller that knows itself to be an
application asks for the detour, since the system cannot read its own attribution without
privilege.

A server created this way SHALL be given the environment it needs EXPLICITLY, because a
process created this way inherits none of the user's — measured as a bare system path and
no language at all. The path is what fails visibly today: without it nothing the server
starts can find tmux, git or any agent binary.

Restoring the working set takes precedence over the attribution: if the system cannot
create the server with its own identity, it SHALL create it the previous way rather than
fail. A working set that returns under the wrong name is better than one that does not
return.

#### Scenario: A permission prompt raised by an agent's own work

- **WHEN** an agent running in a restored server touches data macOS protects
- **THEN** the prompt names the agent's own process tree, not gtmux

#### Scenario: A restore started from a terminal

- **WHEN** the restore is not started by an application that identifies itself as one
- **THEN** the server is created exactly as before

#### Scenario: The server can still find the tools

- **WHEN** a server is created with its own identity
- **THEN** it holds the caller's search path, so the agents and tools it starts resolve

#### Scenario: The privileged path is unavailable

- **WHEN** the server cannot be created with its own identity
- **THEN** the server is created as before and the restore proceeds
