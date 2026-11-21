## ADDED Requirements
### Requirement: Managed HQ reports retain a delivery opportunity during decision pressure
A prospective worker report addressed to HQ with exact managed task identity SHALL remain durable and MAY receive one protected native attention opportunity. Reports with exact same-source turn end evidence take priority. Other managed reports allow a short grace interval before they qualify. This opportunity SHALL neither evict nor trim existing decision entries, and SHALL use established identity, rotation, draft and receipt guards. The automation SHALL not infer urgency or instructions from the report body, grant authority, mark it consumed or replay a possibly submitted generation. Unclaimed recent reports that cannot make progress SHALL expose exact pull references through an independent, rate-limited alert.
#### Scenario: A full decision queue and a recent report
- **WHEN** the regular queue is full, a new managed report remains unclaimed, and a verified native input window is available
- **THEN** the report reference can join a guarded delivery without dropping a decision, and the original report remains unclaimed until HQ claims it
#### Scenario: Several reports remain pending
- **WHEN** earlier notices are still rate-limited or a deferred report reaches its next check
- **THEN** unsurfaced references receive bounded notice opportunities, and due rechecks use ordinary reminders without renewing the exact report's native reservation

#### Scenario: Unknown delivery or a blocked input window
- **WHEN** a protected delivery might already have submitted, or HQ cannot safely receive it
- **THEN** its exact evidence is retained, no duplicate delivery is scheduled, and a later independent notice identifies the unclaimed report
