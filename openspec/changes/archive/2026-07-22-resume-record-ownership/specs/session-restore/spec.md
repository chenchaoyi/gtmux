# session-restore — delta

## ADDED Requirements

### Requirement: Only a real conversation may claim a pane's resume record

The system SHALL NOT let a session take over a pane's resume record unless that session
is a real conversation — one whose log yields at least one parsed turn. This is required
because an agent runs parts of its own machinery as SEPARATE sessions in the SAME pane
(a slash command such as `/usage` gets its own session id and fires the same hooks), so a
differing session id is not evidence that the pane changed hands. Recording such a stub
makes the pane's chat history read as empty and, worse, makes restore relaunch the stub
after a reboot instead of the conversation that was running — losing access to the work
through the very record meant to preserve it. A session already recorded for the pane
SHALL always be able to update its own record, and a real conversation SHALL still be
able to take over a pane, so a genuine handover is unaffected. When the recorded
conversation's log no longer exists, the system SHALL allow the claim regardless, since a
record that can never be resumed must not pin the pane.

#### Scenario: A command stub does not steal the pane

- **WHEN** a slash command runs as its own session in a pane already recorded for a live
  conversation
- **THEN** the pane's record still names the live conversation, so chat history and
  restore both continue to point at the work

#### Scenario: A genuine handover still works

- **WHEN** the user starts a new conversation in a pane that was recorded for an older one
- **THEN** the new conversation becomes the pane's record

#### Scenario: The same conversation keeps reporting

- **WHEN** the session already recorded for a pane fires another hook
- **THEN** its record is updated, without inspecting any log

#### Scenario: A record pointing at a vanished conversation is replaceable

- **WHEN** the recorded conversation's log no longer exists on disk
- **THEN** another session may claim the pane, rather than the pane staying bound to a
  conversation that can never be resumed
