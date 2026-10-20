## ADDED Requirements

### Requirement: Push reconciliation follows foreground lifecycle
Push registration and unregistration reconciliation SHALL cancel its current request when the app becomes inactive or backgrounded, SHALL stop queued alternate-route attempts, and SHALL remain pending for the existing foreground retry. Cancellation SHALL NOT be recorded as a request timeout. The existing write queue SHALL retain its barrier until the transport settles; cancellation SHALL NOT release a still-pending write and allow an older notification preference to overtake a newer one. Real foreground deadline and HTTP failures SHALL remain observable. This SHALL NOT change notification permission or pairing authority.

#### Scenario: Suspension during push registration
- **WHEN** the app leaves the foreground while registration is pending
- **THEN** the request is cancelled, no alternate route starts, and foreground reconciliation can retry
