## MODIFIED Requirements

### Requirement: The pairing window explains an unreachable address from tunnel status

When the "Pair your phone" window cannot reach its own pairing address, it SHALL use
fresh `status/tunnel.json` as evidence of the connector state for either backend.
A fresh connected state SHALL NOT be treated as proof that phones can connect;
the window SHALL say the address has not been verified. A down state SHALL show
its recorded error without claiming that every other network is unreachable.
Stale or missing status SHALL mean the state is unknown. The window SHALL NOT read
logs to decide. A successful gtmux health response SHALL verify the address.

#### Scenario: Direct on a network that hijacks DNS

- **WHEN** the backend is Direct, the window's probe fails, and `status/tunnel.json`
  reports `down` with a resolver error
- **THEN** the window reports the failed connection check with that error
- **AND** it does not tell the user that a phone on cellular connects

#### Scenario: Connected evidence without verified reachability

- **WHEN** the window's probe fails and `status/tunnel.json` reports a fresh `connected` state
- **THEN** the window says the tunnel reports a connection but the address has not been verified
- **AND** it suggests checking the network or proxy without claiming cellular works

