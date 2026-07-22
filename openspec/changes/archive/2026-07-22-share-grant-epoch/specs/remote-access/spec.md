# remote-access — delta

## ADDED Requirements

### Requirement: Pane grants are valid only for the tmux server they were made against

The system SHALL bind guest pane grants to the identity of the tmux server they were
granted against, and SHALL REFUSE those grants while a different tmux server is running,
until the owner grants again. This is required because a tmux pane id is unique only
within one tmux server lifetime: after a restart the ids are reassigned, so a stored id
addresses a different pane than the owner shared. Refusal SHALL apply to every guest path: attaching (before any PTY is spawned),
sending input, reading pane content, and the guest's agent list (which SHALL be empty
rather than risk revealing a pane the owner never shared). Grants carrying no recorded
server identity SHALL be treated as invalid. When no tmux server identity is available the
system SHALL NOT treat grants as invalid. The system SHALL report this state to the owner
so they can re-grant, rather than failing silently. The system SHALL NOT automatically
re-map grants onto a new server by session name, since a name can be reused or renamed and
an automatic re-map could grant access to the wrong session.

#### Scenario: Grants from a previous tmux server are refused

- **WHEN** a guest whose link was scoped before a reboot tries to view or type into a pane after restore
- **THEN** the request is refused and the guest is not shown any pane, because the stored pane ids can no longer be proven to mean what the owner shared

#### Scenario: Re-granting restores access

- **WHEN** the owner grants pane scope again against the running tmux server
- **THEN** the grants are bound to that server and the guest's access works normally

#### Scenario: The owner is told, not left guessing

- **WHEN** grants were made against a different tmux server
- **THEN** the share status reports the grants as stale and tells the owner to re-grant
