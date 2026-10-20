## Implementation
- [x] Cancel push sync across background transitions while retaining the write queue until the transport settles.
- [x] Verify lifecycle cancellation, pending reconciliation and real deadline/failure boundaries.
- [x] Update bilingual documentation, specifications and run the mobile gate.

## Acceptance
Synthetic lifecycle tests do not prove APNs delivery or physical-device background execution. Route probes from the Mac do not attest the phone network.
