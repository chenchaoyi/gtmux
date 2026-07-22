# mobile-app — delta

## MODIFIED Requirements

### Requirement: The supervisor opens a HQ command center, not the generic detail

When the user opens a `role:"supervisor"` session on mobile, the app SHALL present a
dedicated HQ command center — NOT the generic Chat/Terminal detail — and that command
center SHALL be built from what only the supervisor knows, NOT from a second rendering of
the radar. It SHALL NOT list the fleet session-by-session: the per-session list belongs to
the radar, and repeating it here adds no information (fleet COUNTS remain in the status
strip). It SHALL contain, in order: a status strip (fleet counts + subscription-window %
+ resource warning); an ASSESSMENT zone (a deterministic one-line conclusion about what
needs the user, plus access to the supervisor's own situation board with its freshness);
and three switchable zones, each given the full body height rather than a share of it: a
YOUR-CALL zone (one decision card per waiting session, each showing that session's ask as
the card's body rather than as a footnote, and offering both opening that session directly
and asking the supervisor to draft a reply), an ACTIVITY zone (the event ledger at notable
severity and above), and a CONSOLE zone (a conversation with the supervisor). The command
bar — free text plus quick-command chips — SHALL remain available on every zone, since the
user can always have something to say to the supervisor. The zone selector SHALL carry
each zone's own signal (how many decisions are pending, whether activity is new) so the
zones the user is NOT looking at still report themselves. The app SHALL open on the
your-call zone when something is waiting and on the console otherwise, because the reason
to open HQ while a session is blocked is the block. Commands are HQ-mediated: the command
bar addresses the supervisor, which drives the fleet; the HQ screen has NO direct-send
input of its own — direct control lives in each worker's own detail. Every zone SHALL
state its empty condition in words; NO zone may render as a bare header over blank
space.

#### Scenario: Open the supervisor

- **WHEN** the user taps the gtmux HQ card (a `role:"supervisor"` row)
- **THEN** the HQ command center opens with the assessment, your-call, activity and
  console zones, not the generic Chat/Terminal segmented detail

#### Scenario: The fleet is not listed twice

- **WHEN** the user is in the HQ command center with several sessions running
- **THEN** no per-session fleet list is shown, and the sessions are represented only by
  the counts in the status strip and by decision cards for those actually waiting

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

### Requirement: The supervisor's own assessment is readable from the app

The app SHALL make the supervisor's situation board readable on the phone, with the time
it was last updated, so the user can see the supervisor's synthesis without opening its
terminal. It SHALL be presented read-only — the board is the supervisor's own working
memory, and the app is not an editor for it. When the supervisor keeps no board, or its
data is unavailable, the app SHALL degrade to the deterministic assessment line rather
than showing an error or an empty panel.

#### Scenario: Read the board

- **WHEN** the user opens the situation board from the HQ command center
- **THEN** the board's content is shown read-only together with how long ago it was
  last updated

#### Scenario: No board yet

- **WHEN** the supervisor has written no situation board
- **THEN** the assessment zone shows the deterministic conclusion alone, with no error
  and no empty panel
