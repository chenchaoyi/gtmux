## ADDED Requirements

### Requirement: Reply choices provide immediate local feedback
Selecting a current human-input choice SHALL prepare the same matter composer, reveal a prepared state and request input focus without waiting for CLI reads or sending an answer. Existing edited text SHALL be preserved, and obsolete questions SHALL produce explicit feedback. The complete native window SHALL keep the composer reachable when question details are long.

#### Scenario: Choice selected during a slow refresh
- **WHEN** optional session discovery is slow or unavailable
- **THEN** selecting a current question still prepares and focuses its draft without submitting it

### Requirement: Stored workspace reads are independent of optional discovery
An additive stored-only CLI projection SHALL read authoritative matters/overview without roster or history discovery. Native optional source enrichment SHALL preserve newer canonical data and drafts, distinguish unavailable metadata from absent sessions and retain the ordinary full-list entry.

#### Scenario: Source enrichment times out
- **WHEN** optional source discovery fails after stored matters have loaded
- **THEN** the existing matters, questions and draft remain usable and are not replaced by an old full snapshot
