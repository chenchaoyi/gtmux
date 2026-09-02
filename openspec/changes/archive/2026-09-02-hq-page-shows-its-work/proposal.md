# hq-page-shows-its-work — the HQ page is a chief of staff, not a dashboard

## Why

The commander: *"整体太乱太不专业了 … 上部分信息太多，导致下面对话缩在一起；对话过程不可见；
HQ 能力体现不足 … 浪费了 HQ 这么一个能力。"*

Three symptoms, one cause: **the page is organised by data source, not by the
relationship it exists to serve.** Its three zones are three endpoints (digest /
hq-events / transcript) and its header is a fourth data dump. Nothing on it is arranged
around *what I need from my chief of staff right now*.

**The header taxes the only interactive zone.** Estimated from the stylesheet, the
standing header costs ~200pt (status strip 38 + detail 35–50 + assessment 93 + tabs 34),
~260pt with the safe area; the command bar takes ~124pt more. With a keyboard up the
conversation gets roughly 190pt — four or five lines. What it buys with those pixels is
mostly fleet counts and disk/memory, which are one swipe away on the radar, plus a
subscription percentage that drives no decision. The collapsing header does not help
where it matters: in the console it only retracts when you scroll UP into history, so
while you are actually typing you always pay full price.

**The conversation's process exists and is buried.** Steps are parsed and rendered — but
while HQ works you get `正在思考… 42s` and nothing else, and once it finishes its work
collapses behind an 11.5pt toggle in the dimmest colour on the page. Visible when it no
longer matters, hidden while it does.

**HQ's own work is invisible.** Measured over 7 days of this machine's journal:

| | |
|---|---|
| `wake-delivered` | 1532 |
| `knowledge` | 168 |
| `rotate` | 43 |
| `send` | 27 |
| `hq-session` | 34 |
| `self-check` | 8 |
| `reap` | 4 |
| `distill` | 1 |

The supervisor wrote 168 knowledge entries, reclaimed 4 finished dispatches, audited its
own products 8 times and rotated itself when it aged. **None of it reaches the page.**
The ACTIVITY zone reads `severity=notable`, which is the FLEET's lifecycle — sessions
starting, stopping, waiting. So the page shows what the workers did and never what the
chief of staff did. That is the whole "HQ 能力体现不足": the one thing that makes HQ more
than a dashboard is the one thing with no surface.

## What changes

**1. The header carries the judgment and nothing else.** Back + name + connection dot,
then the verdict on one line with a disclosure. ~78pt instead of ~200pt — roughly six
more lines of conversation with the keyboard up.

Everything the header shows today survives inside the disclosure, which opens with
**HQ's own last brief** — the newest `⟣` reply in the transcript — rather than gtmux's
recomputed counts. HQ already produces a periodic brief; a supervisor's own words are
worth more than `3 需要你 · 0 运行 · 12 空闲`. Under it: the fleet line, the resource
line, and the board entry. The resource line is promoted OUT of the disclosure only at
the red tier, which the verdict already models as its `resource` state.

**2. The middle zone becomes what the supervisor DID.** `参谋长做了什么` renders the
`gtmux:audit:*` journal as acts — dispatched, reclaimed, recorded, audited, rotated —
each with its target, what it was, and how it ended, under a week's tally. The fleet
ledger stays, demoted to a filter beside it. "My sessions started and stopped" is not
news; "my chief of staff dispatched work while I was away" is.

No new endpoint: `/api/hq/events` already serves these records at `routine`. What is new
is rendering them as sentences, the same job `eventPhrase` already does for fleet events.

**3. The conversation shows the work as it happens.** The in-flight turn's steps stream
as they land (the phone already polls the transcript with an ETag while the chat is open,
and an agent writes its steps to disk as it goes); the timer stays only as the fallback
when there is genuinely nothing yet. The CURRENT turn's steps are expanded; history stays
collapsed. Archaeology and watching are different needs and had one presentation.

**4. Modularity is part of the change, not a side effect.** `HQScreen.tsx` is 775 lines
of view and logic together. Every decision above lands as a pure module with its own
tests — the brief, the acts ledger, the header model — and the screen becomes composition.
This is the commander's explicit requirement ("模块化一些，增加一些自测，可维护性高一些")
and it is what makes the next iteration cheap.

## Boundaries

- **The radar is not touched.** `HQDisc`, the six-state disc model, and the menu-bar
  parity all stand.
- **No new server endpoint** — but this boundary was written before measuring, and the
  measurement moved it. The acts are sparse in a feed the wake plumbing dominates: the
  200-record cap spans 3.9 hours and carries 5 acts where the day held 37. So
  `/api/hq/events` gains an ADDITIVE `?acts=1`, applied before the cap. Same endpoint,
  same shape, and a client that does not send it is unaffected.
- **The supervisor's recommendation per decision card is NOT in this change.** It is the
  natural next step — a chief of staff should already have an opinion about what is
  blocking you — but it needs HQ to write per-pane advice and a playbook change to teach
  it. Deliberately deferred until the three cheap slices are proven in daily use.
- **Nothing HQ writes becomes editable.** The board stays read-only; the acts ledger is
  an audit trail and is read-only by nature.
