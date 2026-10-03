## ADDED Requirements

### Requirement: Standard tunnel repair preserves pairing identity

The system SHALL support `gtmux tunnel --backend cloudflare --service --force` to
repair an existing Standard tunnel's ingress and DNS without replacing its device
identity, tunnel, or hostname. The control plane SHALL return an explicit repair
receipt. The CLI SHALL refuse an unacknowledged repair before changing service files.
Repair SHALL NOT overwrite a DNS route pointing outside Cloudflare Tunnel.

#### Scenario: Missing DNS for an existing tunnel

- **WHEN** a user forces repair and the registered tunnel still issues a token but
  its DNS record is missing
- **THEN** the control plane reapplies ingress, recreates the proxied CNAME for the
  same hostname, and acknowledges the repair
- **AND** the CLI refreshes the existing services without rotating the device ID

#### Scenario: Repair cannot be completed

- **WHEN** the provider fails, DNS conflicts with an unrelated route, the existing
  tunnel is unavailable, or an older control plane ignores the repair request
- **THEN** the command reports failure and leaves the local service files intact
- **AND** it does not create a replacement tunnel or claim network reachability

### Requirement: One Standard recovery action chooses the repair strategy

The pairing window SHALL expose one **Restore connection** action for Standard
access. It SHALL run `gtmux tunnel --backend cloudflare --service --recover`, repair
an existing usable tunnel in place, and replace it only when the provider confirms
that it is missing or deleted. Provider, authentication and network errors SHALL
NOT trigger replacement. The device ID SHALL remain unchanged in all cases.
The CLI SHALL require a `recovered: true` receipt before changing local services.
The pairing window SHALL verify a successful gtmux health response before reporting
reachability; connector metrics alone SHALL NOT imply that phones can connect.

#### Scenario: Recovery keeps the address

- **WHEN** the original Standard tunnel is usable but its DNS or ingress is damaged
- **THEN** Restore connection repairs the existing route, refreshes the code and
  probes the address without asking the user to choose a strategy or pair again

#### Scenario: Recovery needs a new address

- **WHEN** the provider confirms the registered tunnel is missing or deleted
- **THEN** recovery provisions a replacement under the same device ID
- **AND** the pairing window shows the new address and QR with a re-pairing prompt

#### Scenario: A temporary failure is not a reason to replace

- **WHEN** the provider or network fails without confirming the original tunnel is gone
- **THEN** recovery fails without creating a replacement or changing local service files
- **AND** the menu bar reports failure even when the existing Anywhere mode remains installed

#### Scenario: A connector reports connected but the address does not answer

- **WHEN** the local status reports connected and the public health probe fails
- **THEN** the pairing window says the address has not been verified
- **AND** it does not claim that a phone on cellular can connect
