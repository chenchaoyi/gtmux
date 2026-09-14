## ADDED Requirements

### Requirement: HQ's scripts live in the ledger's folder and are indexed only by entries

Scripts HQ writes SHALL live under `knowledge/tools/`, and each SHALL be named by a live
`howto` entry that says when to run it; the ledger is the only index of what HQ can do.
The charter SHALL define the HQ home's top level as `AGENTS.md`, `LOCAL.md`, `notes/` and
`knowledge/` and SHALL instruct HQ to move an inherited top-level `tools/` or `designs/`
once. `knowledge lint` SHALL report a script under `knowledge/tools/` that no live entry
names as `orphan-tool`, and an entry naming a `tools/<script>` absent from disk as
`broken-tool`; a base with no tools folder SHALL raise neither. gtmux SHALL NOT move
files itself.

#### Scenario: A script nobody wrote down

- **WHEN** `knowledge/tools/hq-nightwatch.sh` exists and no live entry names it
- **THEN** lint reports `orphan-tool tools/hq-nightwatch.sh`

#### Scenario: An entry pointing at a script that left

- **WHEN** a live entry says "run tools/removed.sh" and no such file is under `knowledge/tools/`
- **THEN** lint reports `broken-tool` on that entry
