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
