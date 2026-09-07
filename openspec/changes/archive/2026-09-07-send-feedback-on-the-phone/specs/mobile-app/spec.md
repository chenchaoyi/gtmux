# mobile-app (delta)

## ADDED Requirements

### Requirement: A send that did not land says why, in the core's own words

When `POST /api/send` refuses, the app SHALL show the reason the core gave rather than a
generic failure. The three refusals differ in what the reader should do — someone is
typing in that pane, the pane is gone, the key is not allow-listed — and one bar that
cannot tell them apart sends the reader to the Mac for all three.

No override is offered. `POST /api/send` carries no field for one — the core's
draft-clobber switch is reachable only from the CLI — so an "send anyway" button would
either lie or require a contract change, and a contract change is not a display decision.
The copy says what is true and what the reader can do instead.

#### Scenario: A pane with someone typing in it

- **WHEN** a send is refused because the pane holds unsent text
- **THEN** the app says so in the core's words, keeps the typed message, and does not
  offer an override the API cannot carry

#### Scenario: A pane that is gone

- **WHEN** a send is refused because the pane no longer exists
- **THEN** the app says so, and offers no override

### Requirement: A send into a working session says it will wait

A message sent into a session that is mid-turn is not lost — the agent queues it behind
the current turn — but "delivered" and "queued behind a turn that may run for minutes" are
different facts and the reader acts on them differently. The app SHALL say which it was.

It reads this off the target's status, which the radar already carries, rather than adding
a screen read to a path built to answer immediately. That makes it an inference rather
than a measurement, and it is worded as the expectation it is: the status can be briefly
stale after a missed transition, and a sentence about what will happen next survives that
where a claim about what DID happen would not.

#### Scenario: Sending into a session that is working

- **WHEN** a send lands in a pane whose status is working
- **THEN** the app says the message will be handled when the current turn ends

#### Scenario: Sending into an idle session

- **WHEN** a send lands in a pane that is not mid-turn
- **THEN** the app says nothing extra, because there is nothing extra to say
