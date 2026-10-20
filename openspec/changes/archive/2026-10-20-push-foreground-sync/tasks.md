## Implementation
- [x] Cancel push sync across background transitions and bound waiting even for a transport that ignores abort.
- [x] Verify lifecycle cancellation, pending reconciliation and real deadline/failure boundaries.
- [x] Update bilingual documentation, specifications and run the mobile gate.

## Acceptance
Synthetic lifecycle tests do not prove APNs delivery or physical-device background execution. Route probes from the Mac do not attest the phone network.
