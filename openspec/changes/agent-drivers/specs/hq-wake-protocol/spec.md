# hq-wake-protocol (delta)

## MODIFIED Requirements

### Requirement: Wake delivery is acknowledged, retried, and never silently dropped

A wake SHALL be removed from the delivery queue ONLY after its delivery is confirmed.
Delivery SHALL paste the line and submit it as separate steps (a paste buffer, then a
named Enter key). Confirmation SHALL be layered: when the HQ pane hosts a
hook-equipped agent, a prompt-submit event on that pane containing the batch's
identifier SHALL confirm the delivery deterministically (the driver receipt), with
the screen read retained as the fallback — reading the pane's capture (including
scrollback margin) for the batch's identifier. Any error from the paste or the
submit, and any unconfirmed read, SHALL return the batch to the queue for a later
attempt. A queue entry claimed by a drainer that never completed (a claim older
than 60 seconds) SHALL be reclaimed by the next drain.

A delivery whose paste landed but whose submit did not — the driver receipt reports
the payload unsubmitted, or the screen shows the batch identifier in the DRAFT
region — SHALL be handled as a distinct state: the next drain SHALL re-send ONLY the
Enter for that claim, after confirming the draft still holds that batch's text, and
SHALL NOT re-paste the payload or mint a new identifier. This bounds the
swallowed-Enter failure to one fast-drain interval instead of blocking the channel
behind its own stranded paste until the stale-queue degradation fires.

#### Scenario: A failed send keeps the nudge

- **WHEN** the paste or the Enter of a wake batch returns an error
- **THEN** the batch is returned to the queue and delivered by a later drain, and no
  queue entry is deleted

#### Scenario: An unconfirmed delivery is retried

- **WHEN** a wake batch is pasted and submitted but its identifier does not appear in the
  pane capture
- **THEN** the batch is returned to the queue and re-attempted on the next drain

#### Scenario: An entry that can never be confirmed does not loop forever

- **WHEN** a batch is pasted and submitted successfully but its delivery is never
  confirmed, drain after drain
- **THEN** it is re-sent at most 3 times in total (each carrying the same identifier)
  and then dropped, with the degradation raised — a send that ERRORS instead (nothing
  reached the pane) keeps retrying without limit, since it risks no duplicate

#### Scenario: A crashed drainer's batch is reclaimed

- **WHEN** a drainer claims a queue entry and dies before delivering it
- **THEN** a later drain reclaims the claim once it is older than 60 seconds and delivers
  the entry

#### Scenario: The HQ session's own submit event acks the wake

- **WHEN** a wake batch is pasted and submitted into a hook-equipped HQ pane and the
  pane's prompt-submit event carrying the batch identifier appears on the stream
- **THEN** the delivery is confirmed from the event — the confirmation does not
  depend on finding the identifier in a scrolled capture

#### Scenario: A swallowed Enter is repaired by Enter alone

- **WHEN** a wake batch's paste landed in the HQ composer but its Enter was swallowed
  (no submit event; the identifier sits in the draft region)
- **THEN** the next fast drain re-sends only Enter after confirming the draft still
  holds that batch, the identifier is unchanged, and the payload is never pasted a
  second time — the channel is not blocked until the stale-queue degradation

#### Scenario: A non-hook HQ falls back to the screen ack

- **WHEN** the HQ pane hosts an agent with no driver receipt capability
- **THEN** delivery confirmation uses the screen read exactly as before, unchanged
