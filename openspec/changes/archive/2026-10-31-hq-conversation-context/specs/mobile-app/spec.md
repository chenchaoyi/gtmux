## ADDED Requirements

### Requirement: HQ conversations share one authoritative matter overview
Workspace-capable HQ conversations SHALL use the same current matter counts and overview navigation as the outer HQ page. They SHALL NOT expose a second waiting-session attention tab or inspector. The full context SHALL collapse together, while a critical machine condition remains visible in the compact headline. Expanded context SHALL have a bounded scroll area. Unknown data SHALL NOT be displayed as zero.

#### Scenario: An agent waits without a persistent matter
- **WHEN** digest shows a waiting agent but the authoritative HQ matter set has a different needs-input count
- **THEN** the conversation reflects the matter set and opens the same overview as the outer HQ entrance

#### Scenario: Context is folded
- **WHEN** a reader collapses the context
- **THEN** resource cards and HQ detail rows are hidden, a critical condition remains in the compact headline, and the chat/composer stay mounted

#### Scenario: An older server
- **WHEN** the workspace adapter is absent or explicitly unsupported
- **THEN** the legacy session view remains available as Waiting sessions, distinct from persistent HQ matters

## MODIFIED Requirements

### Requirement: The supervisor opens a HQ command center, not the generic detail

When the user opens a `role:"supervisor"` session on mobile, the app SHALL present a
dedicated HQ command center — NOT the generic Chat/Terminal detail — and that command
center SHALL be built from what only the supervisor knows, NOT from a second rendering of
the radar. It SHALL NOT list the fleet session-by-session: the per-session list belongs to
the radar, and repeating it here adds no information.

The command-center behavior below applies to the existing console and to the legacy
HQ page on servers without the shared workspace read adapter. Current servers open
the matter overview described below; its Talk to HQ action enters this console.

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

Only the legacy HQ page on servers without workspace matters SHALL contain two switchable zones, each given the full body height rather than a
share of it: a WAITING-SESSIONS zone (one decision card per waiting session, each showing that
session's ask as the card's body rather than as a footnote, and offering both opening that
session directly and asking the supervisor to draft a reply), and a CONSOLE zone (a
conversation with the supervisor, with its recorded acts beside its words as required
below). The command bar — free text plus quick-command chips — SHALL remain available on
every zone. The zone selector SHALL carry each zone's own signal so the zones
the user is NOT looking at still report themselves. The legacy app SHALL open on the waiting-sessions
zone when something is waiting and on the console otherwise. Commands are HQ-mediated: the
command bar addresses the supervisor, which drives the fleet; the HQ screen has NO
direct-send input of its own. Every zone SHALL state its empty condition in words; NO zone
may render as a bare header over blank space.

#### Scenario: Open the supervisor

- **WHEN** the user taps the gtmux HQ card (a `role:"supervisor"` row)
- **THEN** the HQ command center opens with the verdict, your-call and console zones,
  with recorded acts beside the console's words, not the generic Chat/Terminal segmented detail

#### Scenario: The supervisor's newest word reaches the header

- **WHEN** the supervisor's newest line in the signal register (`⟣ ⚠`, `⟣ ✅` or `⟣ ◈`)
  sits inside a console turn after other replies, or after a working line in the same reply
- **THEN** the header shows it, because a turn holds every reply the supervisor made after
  the prompt that opened it and its joined text rarely starts with the register; a routine
  register line (`⟣ ▪`, `⟣ 📓`) that is newer still hides it, as before

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
  disclosure, using the same red as the HQ resource badge; amber remains reserved for
  a lower-tier warning in the usage/context view

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

### Requirement: A door on the HQ page shows one fact per line

Each of the three resource tiles (board, knowledge, usage) SHALL split its value into
at most two facts at its " · " joints. Each fact MAY wrap onto two lines on compact
screens, and larger Dynamic Type sizes SHALL stack the tiles. A fact SHALL NOT end in
an ellipsis while its next line is free. With tokens by day available, the usage door's
value SHALL be today's and this week's totals; the tightest window is read in the usage
sheet behind it.

While a door's first fetch is still out, its tile SHALL be drawn in place with the
brand-mark loading placeholder where the value will be (not tappable); the tiles SHALL
NOT appear one by one as their fetches land. A door whose fetch settled with nothing to
show SHALL leave no tile. "Nothing to show" is an answer: an empty result, a serve
without the endpoint (404), or a refusal (401/403). A fetch that failed (a 5xx, a
tunnel's 502, a body that is not JSON, or no response) is not one: the door SHALL keep
what it last showed.

#### Scenario: Opening the HQ page

- **WHEN** the conversation context is expanded and the board, knowledge and usage reads are still in flight
- **THEN** three tiles are already there, each with the loading mark; each turns into its
  value as its read lands

#### Scenario: A read fails after the doors have values

- **WHEN** the board, knowledge and usage reads return 502 after they had returned values
- **THEN** the three doors keep their last values; they leave only when a read answers
  with nothing to show

#### Scenario: A phone-width tile

- **WHEN** the knowledge base holds 499 entries and 6 promotions wait on the commander
- **THEN** the tile reads "499 entries" over "6 waiting on you", both whole

### Requirement: HQ's recorded acts sit in the console, beside its words

Only older servers without workspace matters SHALL retain the two zones, "Waiting sessions"
and "Console". Workspace-capable servers SHALL use the shared matter overview and a
single conversation. There SHALL be no zone of its own for the supervisor's recorded acts (hq-work direction A, 2026-09-15: the tab said
in a fourth place what HQ's own words, the header's "HQ did" row and the Mac card already
said, with nothing to act on). The acts (`gtmux:audit:*` minus wake delivery) SHALL be
drawn in the console as small rows between the turns, each above the first turn that
came after it and the rest after the last turn: time, a dot (cyan for a dispatch or
reclaim, amber for an alarm, dim otherwise), the verb with its target, the detail, and
the worded outcome; a row that leads somewhere (a pane, a knowledge entry) SHALL open it
on tap. Three or more consecutive acts with the same verb inside one hour SHALL fold to
one row (「记账 ×6 ›」) that opens on tap. The header's "HQ did" row SHALL keep the day's
tally and open a sheet listing the acts, whichever zone is open: the purpose sentence,
the last 24 hours' tally, then the acts by day in bursts, each row leading where its act
did. It SHALL NOT merely switch to the console: that was usually the zone already open,
so the tap only selected it again and nothing visible happened (2026-10-05).

#### Scenario: The HQ did row opens the acts

- **WHEN** the Console zone is already open and the reader taps the header's "HQ did" row
- **THEN** a sheet lists the day's tally and the acts, and tapping a dispatch in it closes
  the sheet and opens that pane

#### Scenario: A dispatch beside the claim

- **WHEN** HQ's reply at 23:36 says it sent %9 the commander's decision and the journal
  records the dispatch landing at 23:37
- **THEN** the console shows the reply bubble and, above the next turn, a row
  「23:37 · 派活 → %9 · 司令答了你那批问题里的第一条… · 已送达 ›」 that opens %9
