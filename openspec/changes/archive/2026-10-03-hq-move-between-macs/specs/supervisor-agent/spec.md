# supervisor-agent (delta; proposed migration behavior)

## ADDED Requirements

### Requirement: A new Mac imports selected HQ content without restoring old host state

The system SHALL offer a selective move flow separate from full `hq --import`. It SHALL
read a supported encrypted HQ archive or a plain phone copy, preview its contents without modifying the
destination, and let the user choose the knowledge base and the user's `LOCAL.md`
independently, with neither preselected. It SHALL NOT import the old situation board, managed `AGENTS.md`,
session or event history, machine instructions, connectivity state or credentials.
It SHALL clearly identify and require acknowledgement of an unencrypted source.

The flow SHALL validate the complete input and stage selected files before applying any
change. It SHALL preserve the destination on decryption, validation or apply failure,
and SHALL produce an auditable receipt without copying secret content into logs.

#### Scenario: A new Mac starts with its own work

- **WHEN** the user moves knowledge and instructions from an old Mac
- **THEN** the new HQ keeps its own board and generated instructions, and selected durable
  content is staged and applied only after a preview and confirmation

#### Scenario: Import fails midway

- **WHEN** the archive is corrupt, the passphrase is wrong or an apply step fails
- **THEN** the destination's existing HQ content is unchanged or restored from its
  pre-apply snapshot, and the failure identifies the step

### Requirement: Imported knowledge is reviewed before use on another Mac

The system SHALL preserve imported ledger history and provenance, skip exact duplicates
and put conflicting IDs before the user for resolution. Imported entries SHALL be marked
as requiring review and SHALL NOT be used as facts about the new Mac, dispatched to
agents, or promoted until accepted. Sensitive entries SHALL require a separate opt-in;
tool attachments SHALL remain inert until individually checked.

#### Scenario: An old machine-specific lesson appears in the archive

- **WHEN** the importer sees an entry with a machine audience or an old path
- **THEN** it remains available for review but cannot silently become a rule on the new
  machine

#### Scenario: The destination already has the same entry ID

- **WHEN** the destination's entry is identical to the source
- **THEN** the duplicate is skipped; when its content differs, the importer reports a
  conflict and preserves both versions for review
