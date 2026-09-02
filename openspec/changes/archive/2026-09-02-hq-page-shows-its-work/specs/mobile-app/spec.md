# mobile-app (delta)

## ADDED Requirements

### Requirement: The HQ page shows what the supervisor DID, not only what the fleet did

A supervisor that acts on the user's behalf and never shows its work is indistinguishable
from a dashboard. Measured over one week on a real machine, the supervisor dispatched
work 27 times, reclaimed 4 finished dispatches, recorded 168 knowledge entries, audited
its own products 8 times and rotated its own aged session — and none of it was readable
from the app, whose activity zone showed the FLEET's lifecycle instead.

The HQ page SHALL present the supervisor's own acts — dispatch, reclaim, record, audit,
rotate, and alarms about the supervision itself — as a first-class zone, each act naming what it
acted on, what it was, and how it ended, with a tally over a recent window. The fleet's
lifecycle ledger SHALL remain available beside it as a filter rather than as the default,
because a session starting or stopping is not news and a chief of staff dispatching work
is.

These acts SHALL be read from the journal the core already serves, narrowed by the core
BEFORE its record cap: the acts are sparse in a feed the wake plumbing dominates, and a
client filtering after the cap sees hours where the reader needs a week. Wake DELIVERY is
not an act — it is the core knocking on the supervisor's door, not the supervision
working. An act kind the client does not recognise SHALL still render as a
readable row rather than a raw token, so a newly journalled kind degrades instead of
leaking an identifier at the user.

#### Scenario: The supervisor dispatched work while the user was away

- **WHEN** the supervisor dispatched a task to a worker and the user opens the HQ page
- **THEN** that dispatch appears as an act naming the worker, what was sent and whether
  it landed

#### Scenario: The fleet ledger is still reachable

- **WHEN** the user wants the session lifecycle rather than the supervisor's acts
- **THEN** a filter beside the acts shows the fleet ledger, without either view dropping
  records the other holds

#### Scenario: An unrecognised act kind

- **WHEN** the journal carries an act kind this client predates
- **THEN** the row still renders in words rather than as a raw event token

### Requirement: The HQ conversation shows the work while it happens

A supervisor can take minutes over one turn. Showing only an elapsed timer while it works
and then folding its steps behind a dim toggle once it stops presents the process exactly
when it cannot be watched and hides it while it can.

The HQ console SHALL surface the in-flight turn's steps as they arrive, falling back to
the elapsed-time line only while there is genuinely nothing yet — never inventing an
elapsed time it does not have. The CURRENT turn's steps SHALL be shown expanded and
history SHALL stay collapsed: watching and archaeology are different needs.

#### Scenario: The supervisor is working and has taken steps

- **WHEN** the supervisor is mid-turn and has produced steps
- **THEN** the console shows those steps as they arrive, rather than only an elapsed timer

#### Scenario: The supervisor is working and has produced nothing yet

- **WHEN** the supervisor is mid-turn with no step recorded
- **THEN** the console shows the elapsed-time line, and shows no duration at all when the
  start time is unknown

#### Scenario: The turn finishes

- **WHEN** the turn completes
- **THEN** its steps collapse to the history presentation

## MODIFIED Requirements

### Requirement: The supervisor opens a HQ command center, not the generic detail

When the user opens a `role:"supervisor"` session on mobile, the app SHALL present a
dedicated HQ command center — NOT the generic Chat/Terminal detail — and that command
center SHALL be built from what only the supervisor knows, NOT from a second rendering of
the radar. It SHALL NOT list the fleet session-by-session: the per-session list belongs to
the radar, and repeating it here adds no information.

**The standing header SHALL carry the supervisor's judgment and nothing else** — the
page's identity, its connection state, and the one-line verdict. Everything else the
header once carried standing (fleet counts, subscription window, resource line, the board
entry) SHALL move behind a disclosure on that verdict, because the interactive zone below
pays for every standing pixel and a keyboard leaves it only a few lines. The disclosure
SHALL open with the supervisor's OWN most recent brief where it has one, its words rather
than recomputed counts, since the supervisor already produces a periodic brief and a
tally of states is the weaker answer to "what is going on". A resource condition SHALL be
promoted OUT of the disclosure only at the critical tier, which the verdict already
models.

It SHALL contain three switchable zones, each given the full body height rather than a
share of it: a YOUR-CALL zone (one decision card per waiting session, each showing that
session's ask as the card's body rather than as a footnote, and offering both opening that
session directly and asking the supervisor to draft a reply), a zone for the SUPERVISOR'S
OWN ACTS with the fleet ledger available beside it, and a CONSOLE zone (a conversation
with the supervisor). The command bar — free text plus quick-command chips — SHALL remain
available on every zone. The zone selector SHALL carry each zone's own signal so the zones
the user is NOT looking at still report themselves. The app SHALL open on the your-call
zone when something is waiting and on the console otherwise. Commands are HQ-mediated: the
command bar addresses the supervisor, which drives the fleet; the HQ screen has NO
direct-send input of its own. Every zone SHALL state its empty condition in words; NO zone
may render as a bare header over blank space.

#### Scenario: Open the supervisor

- **WHEN** the user taps the gtmux HQ card (a `role:"supervisor"` row)
- **THEN** the HQ command center opens with the verdict, your-call, acts and console
  zones, not the generic Chat/Terminal segmented detail

#### Scenario: The standing header does not crowd the conversation

- **WHEN** the user is typing to the supervisor
- **THEN** the standing header is the identity row and the verdict line only, with the
  fleet counts, resource line and board entry reachable through the verdict's disclosure

#### Scenario: The supervisor's own brief leads the disclosure

- **WHEN** the supervisor has produced a brief and the user opens the disclosure
- **THEN** the supervisor's own most recent brief is what it opens with, and the derived
  counts follow it

#### Scenario: A machine at its critical tier

- **WHEN** the machine reaches its critical resource tier
- **THEN** the condition is stated in the standing header rather than only inside the
  disclosure

#### Scenario: The fleet is not listed twice

- **WHEN** the user is in the HQ command center with several sessions running
- **THEN** no per-session fleet list is shown, and the sessions are represented only by
  the counts in the disclosure and by decision cards for those actually waiting

#### Scenario: A waiting session's ask is the decision

- **WHEN** a session is waiting on the user
- **THEN** a decision card names it and shows its ask as the card's body, and offers
  opening that session directly as well as asking the supervisor to draft the reply

#### Scenario: Nothing needs the user

- **WHEN** no session is waiting
- **THEN** the your-call zone says so plainly instead of rendering empty

#### Scenario: A zone reports itself while hidden

- **WHEN** two sessions are waiting and the user is on the console zone
- **THEN** the your-call zone's selector still shows that two decisions are pending

#### Scenario: Opening HQ while blocked

- **WHEN** the user opens HQ and at least one session is waiting
- **THEN** the your-call zone is the one shown first

#### Scenario: Selecting a decision card targets a command

- **WHEN** the user selects a decision card
- **THEN** per-target quick actions (e.g. continue / inspect / reply-for-me) become
  available in the command bar, addressed to that session through the supervisor
