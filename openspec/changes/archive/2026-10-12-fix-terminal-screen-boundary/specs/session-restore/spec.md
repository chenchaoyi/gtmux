## ADDED Requirements

### Requirement: Managed direct Codex terminals preserve future scrollback

Direct interactive Codex commands constructed for managed spawn, HQ and standard
restore/adopt SHALL default to `--no-alt-screen`, inserted before subcommands and
session identities. Explicit screen options and custom launcher recipes SHALL
remain unchanged. The resume identity registry/parser SHALL retain compatibility
with older command lines. No running client or global Codex configuration SHALL
be modified; previously unrecorded terminal frames SHALL NOT be claimed recovered.

#### Scenario: A standard Codex conversation resumes after restore

- **WHEN** the saved record names direct Codex and a known session ID
- **THEN** its existing cwd guard and exact session ID are preserved, and its future terminal output uses inline mode
- **AND** a custom wrapper retains its own recipe instead of receiving additional Codex flags
