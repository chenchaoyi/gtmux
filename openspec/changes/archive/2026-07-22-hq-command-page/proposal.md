# hq-command-page

## Why

Opening gtmux HQ on the phone lands on a page whose largest zone re-lists the radar you
just came from. The "fleet board" renders one row per session — status, window, session,
agent, goal, ask, ctx%, tokens — and every one of those sessions is already on the radar
screen one tap away. It adds no information; it only makes the list smaller (capped at
264pt) and pushes the actual conversation down.

The reporter named this directly, twice. First about the card ("HQ 里的 fleet 有点鸡肋,
看上去并没有比普通 session 列表提供什么额外的信息") — which `hq-meta-layer` fixed for the
CARD by replacing the anonymous pips with a synthesized headline, but left the PAGE
untouched. Then about the page: "fleet 信息还存在, 只不过变成了一个可折叠的空白。"

That second sentence is the real indictment. The board is collapsible, and collapsing it
leaves a bare header strip — so the interface's own answer to "this content is redundant"
is to hide it, leaving dead furniture behind. A zone whose best state is collapsed should
not be a zone.

Meanwhile HQ holds three things **nothing else in the product surfaces**, and none of them
are on its page:

1. **Its situation board** (`notes/board.md`) — the synthesis HQ maintains by hand so its
   picture of the fleet survives a context reset. The single most considered assessment of
   what is going on anywhere in gtmux, readable only by opening HQ's terminal.
2. **The severity-tagged event ledger** (`events.jsonl`) — the fleet's history at
   notable/important tiers. The radar shows only the present instant.
3. **Which sessions are actually blocked on the user, and on what.** The digest already
   carries each waiting session's `ask`, and today it renders as line 3 of a cramped row,
   two lines of 12pt text under the goal — the single most decision-dense field in the
   product, formatted as a footnote.

So the page shows what the user can already see and hides what only it can show.

## What Changes

**The fleet board is removed, not collapsed.** Fleet counts stay in the status strip
(already there); the per-session list belongs to the radar, which owns it.

**The page is rebuilt around the three questions the radar cannot answer** — what is the
situation, what needs me, what happened — over the command console that was already right:

- **判断 / Assessment** — a deterministic one-line conclusion (single-source with the HQ
  card's `fleetHeadline`), plus HQ's own situation board behind a freshness line
  ("态势板 · 2h 前"). Tapping opens the board full-screen, read-only.
- **该你拍板 / Your call** — one decision card per waiting session, with the `ask` promoted
  to the card's body instead of a footnote, and two actions: open that session directly,
  or ask HQ to draft the reply. When nothing is waiting the zone states that plainly
  rather than rendering empty.
- **舰队动态 / Activity** — the notable-and-above event ledger as a readable feed. Collapsed
  by default, but its header always carries the newest line, so collapsed is never blank.
- **对话 / Console** — unchanged (ChatView + quick chips + Composer). Target selection now
  comes from tapping a decision card rather than from the deleted board.

**Two owner-only endpoints** are added, because that HQ-only data has no API today:

- `GET /api/hq/board` — HQ's situation board text + last-modified time.
- `GET /api/hq/events?severity=&limit=` — the event ledger at a severity floor.

Both refuse a guest, like `/api/digest` and `/api/usage`: they expose the whole fleet and
HQ's private assessment, which are owner surfaces and never part of a shared scope.

## Impact

- Specs: `mobile-app` (the HQ command page), `remote-access` (the two endpoints).
- Code: `internal/server` (endpoints), `internal/app` (deps wiring), `mobileapp/src`
  (HQScreen rewrite, client, demo client).
- Docs: `docs/design/MOBILE.md` §17 (the mirror of this page), `api/contract.md`.
- Not changed: the menu-bar HQ card and the radar HQ card — `hq-meta-layer` already
  settled those, and this change deliberately does not re-open them.
