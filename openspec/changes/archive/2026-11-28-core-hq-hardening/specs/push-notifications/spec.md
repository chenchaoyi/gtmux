## ADDED Requirements

### Requirement: Asynchronous badge fan-out, APNs token hygiene, and bounded collapse IDs

`PushManager.OnTally` SHALL dispatch silent badge updates asynchronously, push registration endpoints SHALL reject tokens exceeding 512 bytes, `sendNotification` SHALL retire persisted iOS tokens only when APNs reports `410` or `400` with reason `BadDeviceToken`, `SetStatusSource` SHALL read a short-TTL cached radar snapshot, and native `CollapseID` values exceeding 64 bytes SHALL be SHA-256 hashed.

#### Scenario: Dead iOS token is retired while transient errors keep registration

- **WHEN** the push relay returns APNs `410` or `400` `BadDeviceToken` for an iOS token versus `400` `PayloadTooLarge` or `502`
- **THEN** the dead token is unregistered and persisted while transient or payload-shape errors retain the token
