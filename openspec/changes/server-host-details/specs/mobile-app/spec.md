## ADDED Requirements

### Requirement: The Servers page says what each Mac is

For each Mac paired as owner that answers its reachability probe, the app SHALL ask
`GET /api/host`, keeping the answer for five minutes under the credential (address and token,
never the address alone) and asking again once it is stale while the page is shown, and SHALL
add the Mac's own name and
system ("Studio · macOS 26.1") as a clause on that row's existing status line, never as an
added line. A server row's More options SHALL offer Details: what the phone keeps (the name
given on this phone, the address, the access) and what the Mac reported (names, system and
build, chip and architecture, cores, memory, uptime, gtmux version, how long serve has run,
tmux), leaving out any empty field. A share link SHALL never be asked, and the sheet SHALL say
why the Mac's part is missing: a share link does not include it, the Mac's gtmux is too old
(404), the Mac no longer accepts the credential (401: pair again), or the Mac could not be
reached. An answer given to an owner credential SHALL never be shown on a share link's row.

#### Scenario: An owned Mac answers

- **WHEN** the Servers page probes an owned Mac and it answers
- **THEN** its status line reads like "Available · Studio · macOS 26.1", and the row stays two lines high

#### Scenario: The same address paired again as a share link

- **WHEN** an address whose owner answer is cached is saved again with a share-link credential
- **THEN** that row shows no host clause, and the Mac is not asked

#### Scenario: Details for a share link

- **WHEN** the reader opens Details on a share-link row
- **THEN** the sheet shows the name, address and access, and says a share link does not include the Mac's system details, without asking the Mac
