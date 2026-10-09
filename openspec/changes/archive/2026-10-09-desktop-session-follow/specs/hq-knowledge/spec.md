## ADDED Requirements

### Requirement: Future desktop learning consent
Mining SHALL require HQ and independent knowledge consent before reading a verified desktop conversation. All content records SHALL respect the current consent time. A new consent interval SHALL reset content carry and offsets; incremental processing SHALL preserve verified session identity.

#### Scenario: No consent
- **WHEN** HQ follow alone is enabled
- **THEN** no desktop content is mined

#### Scenario: Future content
- **WHEN** knowledge is enabled after existing conversation activity
- **THEN** only later activity can generate candidates with the correct session ID

#### Scenario: Revocation
- **WHEN** the owner stops follow or knowledge capture
- **THEN** subsequent passes do not mine the conversation and existing records remain
