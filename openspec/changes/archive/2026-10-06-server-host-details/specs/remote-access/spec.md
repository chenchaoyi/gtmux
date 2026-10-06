## ADDED Requirements

### Requirement: The serve describes its machine to the owner

`gtmux serve` SHALL answer `GET /api/host` for an owner credential with what the machine is:
its host name, its Computer Name where the platform has one, its operating system name,
version and build, its architecture, CPU model, core count and memory, its boot time, the
tmux and gtmux versions it runs, and when serve started. A field the platform does not offer
SHALL be empty or absent rather than guessed. A guest (share-link) credential SHALL be
refused with 403, and a request without a valid credential with 401, in both cases without
reading the machine. The values SHALL be read once per serve process, on first use, and every
external command SHALL be time-limited (2s), so only the first request can wait on them.
Values read from files (Linux os-release) SHALL be parsed as data, never sourced or expanded.

#### Scenario: The owner asks what the Mac is

- **WHEN** the owner's phone requests `GET /api/host`
- **THEN** it receives the machine's names, system, hardware, uptime and versions

#### Scenario: A share link asks

- **WHEN** a guest credential requests `GET /api/host`
- **THEN** the serve answers 403 and discloses nothing about the machine
