## ADDED Requirements

### Requirement: Verified native terminal lifecycle outranks UI inference
For a currently verified terminal attachment whose adapter declares lifecycle observation, the radar SHALL derive status from a separate native snapshot published only by its verified hook sender after turn acceptance. Evidence SHALL match the current harness, home, session, pane, foreground and client incarnation. Generic markers, frame changes and CPU activity SHALL NOT override this evidence or fabricate a phase when it is absent. With no reliable evidence, the row SHALL remain neutral running without a fabricated completion time. A later explicit turn boundary in the exact durable DSH session MAY repair a missed hook using the existing incremental catalog. This SHALL NOT change input authority or other adapters' recovery paths.

#### Scenario: Native phases survive repaint
- **WHEN** an accepted native hook reports working, waiting, completion, interruption or failure for the current attachment
- **THEN** the shared radar keeps that phase and its transition time through UI repaint
- **AND** a new native start or resume still produces the real working state

#### Scenario: Evidence is absent or belongs to another binding
- **WHEN** only generic active/finished caches exist, or native evidence belongs to an old attachment, session, home, foreground or turn
- **THEN** that evidence does not claim the current turn's phase
- **AND** stale callbacks cannot revive a settled turn or end a newer turn

#### Scenario: Later explicit durable boundary repairs a missed hook
- **WHEN** the exact current DSH session has an explicit turn boundary later than the observed hook and within the current attachment lifetime
- **THEN** the radar may recover its working or terminal state
- **AND** default history idle and older or unrelated boundaries cannot override a native question

#### Scenario: All surfaces use the shared status
- **WHEN** terminal, menu bar, phone, iPad or Web reads the same radar
- **THEN** each receives the corrected shared lifecycle phase without a surface-specific heuristic
