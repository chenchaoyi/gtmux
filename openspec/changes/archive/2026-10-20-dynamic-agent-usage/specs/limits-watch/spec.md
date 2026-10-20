## ADDED Requirements

### Requirement: Account metrics preserve capability and freshness

The system SHALL distinguish subscription windows, monetary wallets and local
token accounting. It SHALL expose only relevant installed or observed sources,
label recorded plan evidence, and retain historical usage after source removal.
Every metric SHALL retain its own observation scope/time; stale quota windows
SHALL NOT drive current warnings or the tightest-window summary. Native DeepSeek
Account balance collection SHALL use its own service, omit credentials and PII,
reject late results after account switches, and deduplicate verified account
observations. Unsupported API-key/custom-provider balance sources SHALL be stated,
not guessed or rendered as zero.

#### Scenario: An account changes during a balance request

- **WHEN** the native account changes before a balance request finishes
- **THEN** the old result is discarded and cannot populate the new account
