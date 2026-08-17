# CLI & commands

| command | what it does |
| --- | --- |
| `agents [--watch\|--json]` | coding agents across your panes: who's waiting / working / idle, where, and the pane id to jump to |
| `panes [--json]` | EVERY tmux pane (not just agents), tiered agent/plain — the superset behind the pane browser |
| `overview [--popup]` | sessions / windows / panes summary; `--popup` fits a tmux popup |
| `restore [--pick\|--one\|<name>\|--dry-run\|--plan[ --json]] [--resume-agents=auto\|type\|off]` | one terminal tab per session, attach all; optionally relaunch captured agent conversations; `--plan` previews what would come back |
| `focus <name\|pane-id\|--last>` | jump to a session's tab; a pane id (`%N`) lands on that exact pane; `--last` = the most-recently-finished agent |
| `new [name]` | start a new tmux session in a fresh terminal tab |
| `adopt <session_id>…` | move a sensed non-tmux (native) agent session into tmux |
| `doctor [--fix [--yes]]` | health check grouped by concern; on a TTY it offers to fix improvable rows inline; `--fix` is the one-stop setup (hook, set-titles, restore, the app) |
| `install [hooks\|app\|all]` | install what gtmux needs; with no target it asks. `install hooks --agent codex\|cursor\|gemini\|copilot\|kiro\|opencode` wires another agent |
| `uninstall [hooks\|app\|all]` | remove it again; with no target it asks (the two have very different consequences) |
| `serve [--port N]` | read-only HTTP+SSE radar for the mobile app / browser mirror (behind a VPN or tunnel) |
| `tunnel [--backend cloudflare\|self] [--quick] [--service] [--redeem <code>]` | expose the radar from anywhere — Standard (Cloudflare) or Direct (self-hosted / paid); see [phone.md](phone.md) |
| `pair [list\|revoke <id>]` | enroll YOUR OWN devices (full control): one one-time code as phone QR / browser link / a one-line `gtmux attach` |
| `share [new\|set\|link\|on\|off\|revoke <id>\|status]` | scoped, revocable links for collaborators — per-link view/type allowlists (see below) |
| `attach <host\|pair-link\|share-link> [%pane]` | bridge a remote tmux pane's PTY to your local terminal (owner or guest) over the serve WebSocket |
| `devices [revoke <id>\|--push\|--forget-push <id\|orphans\|all>]` | the paired-device roster (alias of `pair list`/`pair revoke`); `--push` inspects, `--forget-push` clears push tokens |
| `app` (alias `menubar`) | launch the menu-bar app (`Gtmux.app`) |
| `update [--check\|--cli-only]` | self-update the CLI + menu-bar app |

Bare `gtmux` prints help; `gtmux --version` prints the version. Output language
follows `--lang=en|zh` (default `en`) or `$GTMUX_LANG`. Everything is invoked
explicitly — no shell hooks, works with any shell.

## `gtmux agents`

```
gtmux agents — 6 agents · 1 waiting · 1 working · 4 idle

⏸ waiting  Claude Code  api:0.0     permission to run tests     %7
⠿ working  Claude Code  web:0.0     refactor auth middleware    %11
✳ idle     Claude Code  worker:0.0  add retry backoff     %8  ✓ latest
✳ idle     Codex        docs:0.0    —                     %1

jump: gtmux focus %7
```

Each row is **status · agent · location · task · pane id**, sorted by urgency.

- **⠿ working** — busy (don't bother it).
- **⏸ waiting** — blocked on **you** for a permission/approval, mid-task; sorts
  to the very top.
- **✳ idle** — finished its turn, your move when ready (not urgent).
- **⚠ errored** (amber) — an idle session that ended on an API/tool error (e.g.
  `Unable to connect to API`), not a clean finish. It's still idle (your move), just
  marked how it ended; the row shows the error summary. In `--json`: `error: true`
  + `error_text`.

`gtmux agents --watch` is a live, auto-refreshing dashboard (built with
[bubbletea](https://github.com/charmbracelet/bubbletea)): polls ~1.5s, **↑/↓**
select, **Enter** jumps to the pane, **r** refresh, **q** quit. `--json` emits
the same data for scripts and the menu-bar app.

### How detection works (not Claude-only)

- **Status** comes from the pane title the agent sets itself. A leading braille
  spinner (`⠋⠙⠹…`, what most agent TUIs animate) = **working**; Claude Code's `✳`
  = **idle**. This generalizes across agents that animate a spinner.
- **Which agent** is matched by foreground command (`claude`, `codex`, `gemini`,
  `aider`, `opencode`, …) or by a name in the title.
- Extend/override via `~/.config/gtmux/agents.json` — a JSON array of
  `{"name","commands","idleGlyph"}`; your entries win over the built-ins.
- A pane is listed only if the agent **process is actually running**. A leftover
  agent title over a plain shell (e.g. a resurrect-restored session never
  relaunched) is **not** counted.
- Agents running **outside tmux** (a bare `codex`/`claude` in a terminal) are
  **sensed** read-only via the same hook — listed under **Elsewhere** with
  `source:"native"`. They have no pane (no jump/reply); a resumable one can be
  pulled into tmux with `gtmux adopt <session_id>`.

`⏸ waiting` and `✓ latest` come from state files written by the
[notification hook](#notification-hook). Without it, agents never show `⏸`;
everything else still works.

## `gtmux panes`

`gtmux panes` lists **every** tmux pane, not just coding agents — the superset of
`gtmux agents`. It's the read-only producer behind the pane browser
(tiered-pane-control): a session → window → pane tree, or `--json` for a structured
array. Each pane carries a `tier` of `"agent"` (a coding-agent pane — classified
exactly as the radar does) or `"plain"` (a shell, editor, dev server, log — anything
else), plus its loc, cwd, current command, title, active flag, and copy-mode flag.

```sh
gtmux panes            # session → window → pane tree; ▸ marks agent panes
gtmux panes --json     # [{pane_id, loc, session, window, pane, cwd, command, title, active, in_mode, tier, agent}]
gtmux panes watch %N   # opt a PLAIN pane onto the radar as a watched row
gtmux panes unwatch %N # remove it
gtmux panes --watched  # list watched pane ids
```

Why a separate command from `agents`: `gtmux agents --json` is a locked contract
meaning "coding agents" — the radar. `panes` is the everything-superset for a browser
that reaches ANY pane. `gtmux focus`/`send`/`attach` already work on any pane id, so a
`tier:"plain"` pane is a first-class focus/type/attach target; it just doesn't get the
agent-only intelligence (digest, 1/2/3 approval, dispatch, HQ).

**The tier capability matrix** — same primitives, different power (surfaces apply it):

| tier | on the radar | view/capture | focus/jump | type/send | attach | intelligence (digest / 1·2·3 / dispatch / HQ) |
|---|---|---|---|---|---|---|
| agent (tmux) | auto | ✓ | ✓ | ✓ | ✓ | ✓ |
| plain tmux pane | opt-in (`panes watch`) | ✓ | ✓ | ✓ | ✓ | — |
| sensed non-tmux agent | "Elsewhere" | — | — | — | — | — (read-only) |

The agent radar is NOT diluted — a plain pane appears on it ONLY when you opt in with
`gtmux panes watch %N`, as a distinct **watched** row (no agent status), and it's
dropped automatically when the pane closes. Guest share scope still gates view/type on
any pane the same way.

## `gtmux digest` + `gtmux hq` — the supervisor (中控)

`gtmux digest` is the fleet at a glance, with MEANING instead of just status —
a formatted, column-aligned table, not a wall of prose:

```
1 needs input · 2 working · 1 completed

needs input (1)
  ⏸ api:0.0        1.Yes · 2.Yes, don't ask again · 3.No             3 opts   4m

working (2)
  ⠿ web:2.0        fix the login-token refresh bug                 working   1m
  ● hq:0.0 ⌂       ctx 92% — approaching limit                          ⚠   3h

completed (1)
  ✳ mobile:1.0     done, tests pass                                          2d
```

A one-line summary of counts by state leads, then a section per state
(needs-you first, then working, then completed, then errored if any) —
each row is status glyph · name · goal/last/ask (truncated to the terminal
width) · a right badge (dispatch status / ask-option count / usage warning) ·
a right-aligned relative time. Every field is assembled deterministically
(zero LLM tokens) from what gtmux
already knows: **goal** = the session's last user prompt, **last** = the tail of
its last reply (both from the agent's own transcript), **asks** = a waiting
prompt's parsed options, plus the errored/background modifiers. `--json` emits
the machine form (also served as `GET /api/digest`). A session gtmux has no
transcript for still renders from radar signals alone — agents don't need to
cooperate. Each JSON row states its own perception tier as `sense`
(agent-drivers): `driver` — the agent's hook feeds its state AND its transcript
feeds goal/last; `partial` — the hook is in but no structured content resolved;
`screen` — pure capture/process inference. Additive and derived from facts the
digest already holds (no new collection); consumers may weight trust by it.

`gtmux hq` opens (or focuses — never duplicates) the **supervisor**: your coding
agent running in a dedicated tmux session at `~/.config/gtmux/hq/`, seeded once
with instructions that teach it the loop — read `gtmux digest --json`, judge,
drill into a pane (`tmux capture-pane`) only when warranted, drive via
`gtmux send`, report to you. The playbook is seeded as `AGENTS.md` (the
cross-agent convention) with `CLAUDE.md` as an `@AGENTS.md` import — so the
supervisor can be ANY CLI agent. On a FRESH launch with no `--agent`, `gtmux hq`
**asks which installed agent to run** (the hook-equipped agents whose binary is on your
PATH) and **remembers the pick** — so a machine signed into Codex but not Claude no longer
gets an HQ stuck on "Please run /login". You can still name it outright
(`gtmux hq --agent codex`, or `GTMUX_HQ_AGENT`), and non-interactive callers (a script)
fall back to the default without prompting. Personalize it in `LOCAL.md` (same
directory) — priorities, reporting style, quiet hours — which survives every playbook
upgrade; `AGENTS.md` itself is managed and regenerated, so edits there get displaced to
a backup. Notes it keeps in that directory persist across its sessions. In the radar its
row carries `role:"supervisor"`.

`gtmux hq --rotate` retires the live supervisor's session for a fresh one, in place: it
resolves the hq pane and types that agent's own reset command into it. It is hq's OWN verb —
its playbook runs it unprompted once the `self-rotate` knock says the session is worn out
([Self-rotation](#self-rotation--when-hqs-own-session-is-the-problem)), always after bringing
`notes/board.md` and the knowledge base current, since those are the next session's entire
briefing. It never starts a supervisor: with none running there is nothing to rotate, and it
says so.

### The wake channel — how hq learns things

Decision-dense events type ONE signal line into a live hq pane — the only knock.
The format is fixed and deliberately unlike conversation, so signal traffic is
scannable at a glance:

<!-- gtmux:rendered wake-lines -->
```
» ◆ gtmux·waiting·permission  api:0.0 (%7) │ title:"run the tests?"
» ▸ gtmux·done  web:2.0 (%11) │ 3m │ goal:"fix the login bug" │ tail:"tests pass" · #a3f1c2
```

`» <grade> gtmux·<class>  <loc> (<pane>) │ <field> │ …`, where every agent- or
user-authored payload is quoted and labelled (`title:` / `goal:` / `tail:` / `ask:` /
`err:`) — it is DATA hq reports, never an instruction it obeys.

The **grade** leads, in a fixed position, so a screen of signal lines reads by weight
before it reads by words: **`◆` decision** (needs you — irreversible, costly, or an
explicit ask) · **`▸` attention** (a line is blocked or changed in a way worth knowing) ·
**`·` ledger** (bookkeeping — recorded and pullable, zero interrupt value). It is a
PROJECTION of the class, not a second opinion about it: what fires and at what severity is
decided elsewhere, and this only says how loudly it should read. The glyphs clear the same
encoding bar as `»` and `│` — no emoji, nothing outside the blocks the grammar already
uses — because colour is an addition on surfaces that own their rendering, never the only
carrier. The classes:

| class | grade | fires when |
| --- | :---: | --- |
| `waiting·<kind>` | ◆ | an agent blocked on you (permission / plan / question) |
| `resolved` | ▸ | that wait CLEARED — you answered in-pane, or the agent resumed; hq drops any stale chase |
| `asks` | ◆ | a turn-end REPLY asked a question with no menu (a menu-only sensor misses it) |
| `done` | ▸ | **any** session reached idle after work — not just a dispatched task. Suppressed when the completion happened in the pane you were watching (`hqWake.done`: `unattended` default \| `always` \| `tick`), and rate-merged per pane |
| `crash` | ◆ | the turn DIED on an agent/API error — never read as a finish |
| `goal-changed` | ◆ | you submitted a prompt straight into an agent's own window (incl. a slash command), so hq senses work it didn't dispatch |
| `new-session` | ▸ | a newly sensed agent pane — enroll it |
| `reap-suggest` | ▸ | a dispatch looks reclaimable · carries the exact `gtmux reap <id>` |
| `stuck·waiting` | ▸ | a pane has been waiting on you past the timeout — once per waiting episode, and only when the AGENT asked (a wait gtmux merely inferred from the screen never escalates) |
| `resource·warn` / `limits·warn` | ▸ | a machine/subscription threshold crossed (damped — see `gtmux resource`) |
| `usage·warn` | ▸ | a session crossed a context/burn layer (see `gtmux usage`) |
| `wake-degraded` | ◆ | perception itself broke: wakes stopped landing on the HQ pane |
| `tick` | · | the periodic brief — only when something actually changed (a quiet interval costs nothing) |
| `distill` | · | a knowledge-distillation pass is due (≈ weekly, sooner once ≥5 `gtmux capture` candidates are queued) — hq folds the period's lessons into its knowledge base and prunes stale |
| `self-check` | · | hq's own housekeeping is due (≈ daily) — ledger archival, feed and memory health |
| `unread` | · | events are sitting past hq's consumption watermark — a count and the cursor to pull from, no importance claim |
| `self-rotate` | ◆ | **hq's own session** is worn out (context / age / turns) — it hands off and rotates itself, see below |

**Standing classes re-check themselves.** The classes that repeat until an act clears them
(`self-rotate`, `unread`, the warns) ask two questions before each REPEAT that a first knock
never has to: does the premise still hold, and has anything changed since I last said this?
A repeat whose breach set and world are unchanged is suppressed and re-arms on any drift,
with a safety floor so a standing debt is never forgotten; and a queued wake is re-sampled
just before it is typed, so one whose premise died while it waited is dropped rather than
delivered as a claim about a world that has moved on.

`distill` and `self-check` are **maintenance** classes: they knock at the lowest priority,
so they never arrive ahead of a blocked agent, and both passes are silent unless hq
actually did something. They are also the only classes raised purely on a clock — hq has
no timer of its own, so `gtmux serve` is what makes the periodic rituals happen at all.

Everything else is **pull-side**: hq wakes, then reads `gtmux events --since-seq <n>`
or `gtmux digest`. Ordinary progress turns never touch its screen.

**hq's replies to signal lines are signal lines too** — one line opening with `⟣` plus a
glyph, so its pane scans the same way its inbox does. The legend:

| reply | means |
| --- | --- |
| `⟣ ✅ <pane> <judgment> → <next step>` | a completion worth knowing about |
| `⟣ ▪ noted: <one clause>` | routine outcome, recorded to the board — nothing owed |
| `⟣ 📓 captured: <topic>` | a durable lesson written into the knowledge base |
| `⟣ ⚠ <escalation>` | something needs YOU, per the escalation policy |
| `⟣ ◈ 简报 <time> │ <counts> │ top item` | the periodic brief (plus up to 5 indented `· ` lines) |

### The watermark — why nothing goes missing

Every class above is gtmux deciding an event is worth a knock, and that decision needs
context gtmux does not have: only hq knows it is waiting on the pane that just finished.
So the classes are **priority labels, not coverage** — they say what to read first. What
guarantees hq hears about something at all is a **consumption watermark**: gtmux records
how far hq has read the stream, and when events sit past it for `hqWake.unreadDebounceSec`
(default **120 s**), it knocks with a count and nothing else:

<!-- gtmux:rendered unread-line -->
```
» · gtmux·unread  7 unconsumed (%21 ×4 · %13 ×2 · control) │ pull: gtmux events --since-seq 6653 --json
```

The line names **what** it counted, by source (`control` = gtmux's own maintenance
triggers, `native` = a non-tmux agent session), most numerous first and folded to
`+N more` past three — so an accumulation dominated by one pane is visible without a pull.

The debt clears only when hq **consumes**, and only two things count: an unfiltered
`gtmux events --since-seq <n>` read from the hq home (the everyday pull-on-wake — which is
why this needs no new habit), or an explicit `gtmux events --ack <seq>`. A
`--severity`-filtered read does not (it showed a subset), nor does one starting ahead of
the watermark (it skipped the range between). Until then the knock repeats every
`hqWake.unreadRepeatSec` (default **300 s**), at standing priority.

Three kinds of record are excluded from the **count**: hq's own lines, or the channel would
feed itself forever; a pane-less lifecycle **blink** — a `SessionStart` with no pane
whose `SessionEnd` follows within seconds, i.e. a short-lived subprocess hq can neither act
on nor attribute; and gtmux's own **audit trail** (`gtmux:audit:*`) — the journal of what
the supervision *did* (each wake batch delivered or dropped and why, every `gtmux send`
settlement, reaps, the hq session rotation chain), which documents acts hq already knows
about, so counting it would mint fresh debt per delivered knock. The blink rule keys on
that Start/End **pairing**, never on the empty pane alone, and the audit rule keys on the
sub-namespace alone: a native (non-tmux) agent's turns and gtmux's non-audit `gtmux:*`
triggers (maintenance, degradations, reconciles) are pane-less too, still count, and for
them this knock is the *only* channel there is — the class wakes all require a pane.

**hq's pull shows that same set** — the debt, not hq's own trail. Measured on a week of one
fleet, 68.7 % of what a knock sent hq to read was its own echo, so a knock about *one* new
fact cost a whole turn to read. The count and the read now name the same records; `--all`
gets the raw view back, and both still count as consumption (one is exactly what hq owes,
the other a superset). Nothing is ever removed from the log, and a non-supervisor read is
returned untouched.

This is what makes perception complete rather than well-guessed. Before it, an event no
class claimed simply never arrived: a session finishing work nobody had dispatched through
gtmux was neither `done` (no ledger entry) nor `asks` (no question), so the turn-end sat in
the stream, correct and unread, until the user asked in person. `gtmux doctor`'s **HQ
maintenance** section reports the lag (`event consumption`), and flags an hq that is 20+
events or 30+ minutes behind — the failure is otherwise silent in both directions.

Delivery guards your draft and confirms itself: a line is never typed into a non-empty
hq input box (it queues to disk and lands when the box clears), and a batch is dropped
from the queue only once it's been seen on screen — so a failed send retries instead of
vanishing. That makes it at-least-once, hence the trailing `#<id>`: the same id twice is
a re-send, and hq's playbook says to ignore it. Every terminal outcome is also journaled
(`gtmux:audit:wake-delivered` with the full batch, `gtmux:audit:wake-dropped` with the
reason — evicted / unconfirmed / superseded), so "what was hq told at 14:02, and what was
it never told" are `gtmux events --all` queries rather than reconstructions.

### Self-rotation — when hq's own session is the problem

Every class above is about the fleet. `self-rotate` is the exception, and it exists because
of a specific failure: **in a long, near-full session hq starts to read its own output as
input that came from outside.** On 2026-08-03 one did exactly that — it found a line saying
"that message was from me, don't worry", took it for the user's reassurance, and dropped a
suspicion it had raised correctly. The line was its own previous turn. The event stream is
what settles it, because the two acts carry different types: on hq's pane
`UserPromptSubmit` is you, `Stop` is hq.

hq cannot catch this itself — the faculty that would notice is the one that degraded — and
it cannot schedule the check either, since it is not running between wakes. So `gtmux serve`
watches from outside and knocks:

```
» gtmux·self-rotate  ctx 82% · 14h · 380 turns │ over: ctx 82% ≥ 75% │ board+KB current → hand off → gtmux hq --rotate
```

Three facts are sensed, and **any one** crossing its line is a breach:

| `hqWake` key | default | what it measures |
| --- | --- | --- |
| `selfRotateCtx` | **0.75** | live context fraction — the same `ctx` `gtmux digest` reports |
| `selfRotateHours` | **12** | session age, from the transcript's FIRST message (a `serve` restart can't reset it) |
| `selfRotateTurns` | **300** | hq-pane prompt submissions since the session began |

Set any of them to **0** to switch that one criterion off without losing the other two.
`selfRotateRepeatSec` (default **1800 s**) paces the re-knock, `selfRotateCheckSec` (default
**300 s**) paces the evaluation. Defaults are deliberately conservative — a knock you don't
believe is worse than no knock.

The debt clears **only when the session actually rotates**, observed as a new agent session
id; delivering the wake does not clear it, exactly like the watermark. It does not RESTATE
itself for nothing either: past `hqWake.selfRotateRepeatSec` a repeat fires only when the
breach set or the fleet has changed, and `hqWake.selfRotateFloorSec` (12 h) is the longest
it can stay silent when nothing has changed at all. Without that, an age breach — which can
never recover — knocked every half hour forever: one measured night cost 17 knocks against
a completely still fleet, with context climbing largely on the answer-the-knock turns
themselves. hq's playbook tells it
to do three things in order and **not** to ask you first: bring `notes/board.md` and the
knowledge base current (they are the successor's entire briefing), record the handoff, then
run `gtmux hq --rotate` — which resolves the hq pane and types that agent's own reset command
(`/clear`, or `/new` for codex) into it. A repeated `self-rotate` after a rotation means the
rotation did not take, not that a second one is owed. `gtmux doctor`'s **HQ session health**
row shows the same figures, so you can check the claim or dispute the thresholds.

`"hqNudge": false` in `~/.config/gtmux/config.json` disables the channel entirely (no hq
pane → no wake, no cost). The wake only INFORMS: gtmux never answers another agent's prompt,
never sends navigation keys into a TUI, and the default policy tells the supervisor to
surface decisions to you, not take them.

## `gtmux capture` — cheap notice into HQ's knowledge base

```
gtmux capture "<one-line lesson> @<topic>"   # topic ∈ the knowledge vocabulary: six built-ins + topics hq declared
gtmux capture --list                         # show the pending-distill queue
```

```
last distill: 3d ago
2 pending-distill candidate(s):
  [pitfalls] wrangler TLS-resets from the office network — retry
```

Writing a polished knowledge-base entry mid-work is expensive and gets skipped, so
`capture` decouples **noticing** (one line, in the moment) from **writing it up well**
(batched, at HQ's distill pass). It is a **public** command by design — any worker, not
just hq, that learns a durable, cross-cutting fact can drop a **candidate** into a
pending-distill spool (`~/.config/gtmux/hq/knowledge/.pending-distill.jsonl`). Each line
carries the lesson, its topic tag, a **dedup key** (`<topic>/<lesson-slug>`, so the
distill pass MERGES same-key candidates instead of scattering near-duplicates), and
auto-collected event context (the current pane, the event high-water mark, a timestamp,
and `$GTMUX_TASK_ID` if the caller is a tracked dispatch).

A candidate is **not** a knowledge-base entry: hq's distill pass is the quality gate that
decides what is durable, files it under the right topic, and prunes — so opening the
input is safe (worst case a candidate is dropped at distill time). This is layer ② of the
capture loop; see `openspec/changes/archive/2026-07-29-hq-capture-loop`.

`--list` heads the queue with **when the queue was last drained**, because the depth alone
can't tell you whether the loop is alive: an empty queue reads the same whether distill
drained it yesterday or has never run. Five queued candidates also **pull the next distill
forward** — a captured lesson is filed within a day rather than waiting out the week.

## `gtmux knowledge` — the knowledge ledger (entries with provenance)

```
gtmux knowledge add --topic pitfalls --title "wrangler TLS-resets; retry" [--body-file -] [--capture <key>] [--seq-range a..b]
gtmux knowledge supersede <id> --title "…" [--body-file -]   # replaces an entry; history stays in the ledger
gtmux knowledge retire <id> --why "…"                        # prune, with a reason that survives
gtmux knowledge dismiss --capture <key> --why "…"            # reject a candidate WITH a trace
gtmux knowledge topic <name> --desc "…"                      # declare your own topic (clients, datasets, …)
gtmux knowledge promote <id> --why "…" [--target "…"]        # charter-level → export brief
gtmux knowledge land <id> --ref "<pr/spec>"                  # close the loop when it lands
gtmux knowledge promotions [--json]                          # the pending export queue
gtmux knowledge list [--topic t] [--json]  ·  show <id>  ·  render [--check]
```

The knowledge base's **authority is an append-only ledger**
(`~/.config/gtmux/hq/knowledge/.ledger.jsonl`); the topic `.md` files are **rendered**
from its live entries — gtmux-owned, marked, and drift-checked (`render --check` catches a
hand edit; `render` restores). Every entry carries **provenance**: the event seq at write,
an optional distill range, and — when it consumed a capture candidate — that candidate's
pane/seq/task and key, inherited instead of evaporating at merge time. `add --capture
<key>` consumes **every** pending same-key candidate into one entry (the merge the distill
discipline promises); `dismiss` removes them with a journal trace, so the quality gate's
rejections stop vanishing identically to its acceptances. Every mutation appends one
`gtmux:audit:knowledge` record to the event journal.

Mutations are accepted **only from the HQ home** (the same cwd-keyed role rule as
`gtmux events --ack`) — the quality gate is the supervisor; workers record candidates with
`gtmux capture`. `list`/`show` work anywhere.

**The topic vocabulary is yours to extend.** Six topics ship built in (accounts,
workflows, best-practices, pitfalls, corrections, environment); `gtmux knowledge topic
<name> --desc "…"` declares your own — a ledger operation like any other, rendered
immediately with its description, honored by `gtmux capture`, every knowledge verb, and
the dispatch-time echo (custom topics join pitfalls/workflows there; accounts /
corrections / environment deliberately stay out of dispatch context). Names are slugs
(`a-z 0-9 -`, ≤ 40 bytes); built-ins, existing topics, and the reserved directory names
refuse loudly. Declarations are add-only for now.

**A charter-level lesson has a mechanical exit** (the thing hq's own ledger once said was
missing): `promote` marks a live entry charter-level and writes a **promotion brief** —
a self-contained hand-off under `knowledge/promotions/` carrying the lesson, the why, the
suggested landing spot, and the entry's full provenance — which a human (or a worker they
dispatch) carries to whichever durable rule carrier fits: a project `AGENTS.md`/`CLAUDE.md`,
a team runbook, `LOCAL.md` (when the rule governs the supervisor itself), or gtmux's own
repo — an openspec change for developers, or simply a GitHub issue with the brief attached
for everyone else. `land --ref` closes the loop (the ref can be a PR, an issue URL, or a
runbook name alike) and removes the brief,
while the ledger keeps the whole lifecycle. Topic renders badge the state (`⚑ promoted
(pending)` / `→ landed <ref>`), `promotions` heads its list with the count and the oldest
age, and `gtmux doctor` flags a brief that has waited past its floor (~2 weeks) — an
un-carried promotion is exactly the rot this exit ends. gtmux never writes into any repo
itself, and nothing auto-dispatches: the queue surfaces, the commander decides.

**Migration is incremental:** the first mutation touching a topic moves its pre-ledger
hand-written file verbatim to `knowledge/legacy/<topic>.md` (an untouched seeded
placeholder is simply replaced), the render links to it, and the dispatch-time knowledge
echo consults BOTH — so nothing loses reach while hq migrates lessons by use.

## `gtmux spawn` / `gtmux tasks` / `gtmux reap` — verified dispatch

`gtmux spawn <goal>` dispatches new work to a coding agent and confirms it actually
LANDED — the supervisor's (and your) reliable way to start a task without hand-rolled
tmux choreography:

```
gtmux spawn "add a --dry-run flag to the deploy script"
gtmux spawn --title fix-auth-mw --worktree feat/dry-run --model opus "add a --dry-run flag"
gtmux spawn --pane %14 "keep going, then run the tests"
gtmux spawn --json "…"   # → {task_id, pane_id, loc, title, session, delivered, state, judged_by, evidence}
```

**Write the goal to a FILE — that is the standard action.** Anything longer than one
short line goes through `--goal-file <path>` (or `--goal-file -` for stdin), and
`gtmux send` has the same channel as `--message-file <path|->`:

```
cat > /tmp/goal.txt <<'EOF'
把 `hqPlaybookVersion` 提到 13，并且：
1. 运行 `make check`
2. 别让 shell 碰 $HOME 或 `for f in *; do echo $f; done`
EOF
gtmux spawn --title bump-playbook --cwd ~/src/gtmux --goal-file /tmp/goal.txt
gtmux send %14 --message-file /tmp/reply.txt
```

The reason is structural, not stylistic: **a goal passed as a command-line argument is
parsed by your shell before gtmux ever sees it.** Inside `"…"` a backticked span is
EXECUTED, `$foo` is expanded, and a newline ends the command — so a goal like the one
above dies with `command substitution: syntax error near unexpected token 'done'` and
dispatches nothing. Any sufficiently long natural-language instruction eventually
contains one of those characters, which is why "quote it carefully each time" is not a
property you can rely on. The file channel has no shell on it at all: the bytes go file
→ gtmux → `tmux load-buffer -` (a pipe) → the agent's input box. Exactly one
normalization is applied and it is specified — at most one trailing newline is stripped,
because every heredoc appends one. Passing both a file and a positional goal is an
error, not a precedence rule. The positional form stays perfectly good for short
instructions (`gtmux spawn --pane %14 "keep going"`).

**Re-running a failed dispatch converges.** A spawn that dies partway used to leave a
worktree the retry then tripped over (`exit status 128`) and an empty session per
attempt. Now: `--worktree` REUSES a worktree that already serves that branch (a path
held by a *different* branch is still a hard error); before creating a session, spawn
looks for its own previous attempt — a ledger entry that owns its session, never got its
goal delivered, and still has a live pane — and adopts that pane instead of parking a
second one beside it, updating the SAME ledger row; and when a step fails with nothing
resumable, a worktree/branch this invocation created is rolled back. Re-running the
identical command therefore lands on one worktree, one session, one ledger entry.

**Window-title standard.** `--title` names the window's PURPOSE — a concise verb-object
kebab slug (`fix-auth-mw`, `review-pr-518`), which becomes the window + pane name across
tmux, the radar, and the app. On success `spawn` reports the **standard handle**
`<loc> (%pane) · <title>` — `loc` is the LIVE tmux locator `session:window.pane` (the
window's number, recomputed each read so it stays correct under `renumber-windows`; it is
never baked into the name). Refer to a spawned window by that `loc %pane · title` so you
can jump by number. The supervisor's playbook requires a concise `--title` on every
dispatch and this handle in every report.

**HQ-home quarantine.** `spawn` refuses to run a worker in the HQ home (an explicit
`--cwd` naming it, the inherited cwd when `--cwd` is absent, or `--pane` reuse of a
pane sitting there): the home's `AGENTS.md` is the supervisor charter, and a worker
launched there would read it and impersonate HQ. Pass `--cwd <project dir>` — the
supervisor's playbook requires it on every dispatch.

It launches the agent (a fresh detached session by default, or `--pane <id>` to reuse
one, or `--worktree <branch>` to run in an isolated git worktree), **through the
network proxy by construction** (never a bare, un-proxied launch that would 403),
waits for the agent to come up, then delivers the task via a tmux **paste buffer** and
**verifies** it landed.

**Waiting for the agent to come up is a real gate, and when it times out it now names
what blocked it.** The pane is ready only once its composer row is drawn, no trust gate
or choice menu is up, no boot banner is on screen, and two captures are byte-identical
(or the agent's session-start event has fired). A boot banner is chrome that RESOLVES BY
WAITING — `Connecting…`, `Loading…`. A **standing notice** is not: `⚠ N MCP servers need
authentication · run /mcp` names an action only you can take and never clears, so it does
NOT hold the gate (it used to, which made spawn impossible on any machine carrying one —
see TROUBLESHOOTING). On timeout the failure reads
`✗ NOT delivered → <handle> — evidence: … blocked by: <the line that said no>` followed
by the pane's bottom region, not its whole scrollback. Verification is layered: for a hook-equipped agent (Claude
Code, Codex, …) it prefers the deterministic `UserPromptSubmit` event on the #388
session-events stream (no screen-scraping) — the event's recorded head and the
verifier's needle come from ONE shared normalization pipeline, so a genuine submit
event always matches; otherwise it falls back to a hardened, two-frame screen-read
that locates the input box structurally. Arbitration is positive-monotonic: a
stream-confirmed landing is final (a screen read can never overturn it), and before
any `delivered:false` the stream is re-read once more so a confirmation arriving at
the deadline is never lost to the timeout. The JSON result says which layer judged
it (`judged_by: driver|screen`), so a misjudgment can be attributed instead of
reconstructed from timelines. Turning the receipt capability off
(`driver.<agent>.receipt: false` or `driver.enable: false` in
`~/.config/gtmux/config.json`) forces the pure screen-read path — a deliberately
more conservative fallback for isolating event-channel faults. The paste is
**bracketed**, so a multi-line instruction lands as ONE draft and the separate Enter
submits it once (sent raw, each newline would reach the TUI as a bare Return and
submit that line on the spot). The ONLY success is a confirmed landing — a swallowed
Enter is re-sent with backoff, a fragment paste is retried, and a timeout reports
`delivered:false` with on-screen evidence, never a silent success. A retry can never
duplicate: the text is pasted at most once, a re-paste happens only into a box
confirmed empty (the clear key empties one line, so a multi-line draft can survive
it), and a paste that merely rendered late is left alone rather than pasted again. A queued submission is reported as `state:"queued"`. A **re-send
interlock** refuses an identical payload to the same pane within a window (so a nervous
duplicate `/compact` can't double-fire); `--force` overrides it. Pre-flight checks
(proxy, machine resource, subscription window) are advisory and never block.

`--oneshot` dispatches a ONE-SHOT, non-interactive worker through the agent
driver's headless mode (`claude -p … --output-format stream-json`, `codex exec
--json`) — accepted only for a headless-capable agent, refused explicitly
otherwise (never silently degraded to an interactive spawn). The goal travels as
an ARGUMENT, so there is nothing to paste and nothing to land-verify; the run
still lives in a tmux pane (its JSON stream visible, its radar row present, reap
applicable), and its lifecycle truth — done / crash — comes from that stream plus
the exit code, never from screen classification. The explicit contract: a
one-shot pane is WATCH-ONLY — you cannot jump in and take over mid-run. Distinct
from `--headless`, which only suppresses the terminal tab: a `--headless` spawn
is still a fully interactive session you can attach to and steer.

`gtmux send <pane> <text>` uses the SAME land-verification by DEFAULT now (returns as
soon as it confirms, so a healthy send stays fast); `--no-verify` opts out,
`--force` overrides the interlock, and `--json` prints the verified result
(`{delivered, state, judged_by, evidence}` — verified sends only).

**A send never writes into someone else's unsubmitted line.** A paste APPENDS to the input
box, so if the pane's owner is mid-sentence at the keyboard, delivering would concatenate
your payload onto their words and submit both. Every path — the CLI, hq, `--no-verify`, and
the phone — reads the draft first and refuses (`state:"refused-draft"`, nothing written to
the pane) with the draft quoted back. `--force` waives it, and deliberately not the phone's
idempotency key — a send from another device is the one with nobody present to undo it.

The check is deliberately narrow, because **its job is to protect a send, never to block
one.** It runs only on a pane a KNOWN AGENT drives — a pane running vim, ssh or some other
TUI has no composer, and reading one there returns the transcript as a "draft". It reads the
COLOUR capture, so the agent's own dim suggested-next-command isn't mistaken for your typing,
and it wants to see the same thing in TWO frames before refusing. Everything it cannot judge
lets the send through:

| what it sees | what it does |
|---|---|
| no agent in the pane (no composer) | sends |
| the capture fails — tmux hiccup, pane gone | sends |
| the pane is scrolled (copy-mode) | sends |
| a plain shell, no input box | sends |
| your own text — the same message being re-sent | sends (idempotent) |
| someone else's text, seen twice | **refuses** |

It costs at most two reads and one poll interval, and never loops — so it can slow a send by
a known amount, but it can't hang one.

A send that ends `failed` may be **retried immediately**: the interlock drops the record of
an attempt that never landed, so the obvious next move is no longer answered with
`refused-duplicate`. A `queued` send keeps its record — the agent already took it. What `--no-verify` and the mobile `POST /api/send`
skip is the CONFIRMATION, not the mechanics: every text path pastes and then sends
Enter as its own key, so an unverified send can't split a multi-line message either.
`--key` remains a single keystroke. For anything longer than a short line, use
`--message-file <path|->` for the same reason `spawn` has `--goal-file`: text passed as
an argument has to survive your shell first.

A **plain terminal pane** — no coding agent in the pane's process subtree, a bare
shell (bash/zsh/…) in the foreground — is typed into DIRECTLY: text then Enter, no
input-box confirm and no re-send interlock (running the same shell command twice is
normal usage, not a double-dispatch). There is no agent composer to verify against,
and running the box-confirm against a shell false-failed whenever stale box-drawing
sat in the pane's scrollback (a pane that used to run an agent). `--json` reports such
a send as `{"delivered":true,"state":"sent"}` — sent, with nothing to land-verify.
Anything not provably a plain shell (vim, ssh, an agent the process scan missed)
keeps the agent pipeline.

`gtmux tasks [--json]` is the **dispatch / needs-you ledger**: every task you spawned
with its live status (undelivered / waiting / done / working / gone), needs-you first.
`--verbose` adds archived entries and the attention columns (tier · priority · surfaced ·
disposition).

**`undelivered` leads the list, and it is the one status the pane cannot tell you.** A
dispatch that dies at the ready gate leaves a live, EMPTY, idle agent pane —
indistinguishable from one that just finished a turn — so a status derived from the pane
alone rendered a task that never started as green `done`, with the goal you *intended*
printed beside it. The ledger records the delivery verdict and `tasks` respects it: an
entry whose goal never reached the agent reads `✗ undelivered` however idle its pane
looks. A `queued` delivery is not undelivered (the agent accepted it, behind the current
turn), and a rescue that works closes the record — a `gtmux send` that LANDS that same
goal in that pane marks the entry delivered, so the workaround below doesn't leave a
permanently wrong row. Only that goal: an unrelated line typed into the pane doesn't
launder a dispatch that never happened.

`gtmux tasks --pending` is the **pending-decision standing view** — the one home of
what is on your plate:

```
◆ t1kx8p2m9dq3  hq %21                 08-09 14:32  ship v0.48.0 or hold for §4?
▸ t1kx9r4w0aa1  worker-b %8            08-09 09:11  which branch should the migration target?
```

The leading glyph is the attention grade (`◆` decision · `▸` attention · `·` ledger),
projected from the entry's tier — the same scale the wake lines carry. Ordering is
total and stable: decision grade first, then the oldest wait, then pane, then id. The
view reads the LEDGER ONLY (no radar scan) and prints an ABSOLUTE stamp rather than a
"waiting 3h" countdown, so two reads of an unchanged plate are byte-identical — it
moves only when the set does. That is what lets a brief point at it (「其余照旧」)
instead of re-printing the list every time.

Entries go on and off the plate with `gtmux tasks --await <task_id>` and
`gtmux tasks --resolve <task_id> [disposition]` (the disposition records how it left —
`decided` / `withdrawn` / `escalated`; omitted, it just clears). Membership is the
`awaiting-commander` disposition and nothing else, so any other disposition also takes
an item off the plate, and archiving an entry closes it out of the view.

`gtmux reap <pane|task_id>` safely reclaims a finished dispatch — it runs a safety gate
FIRST (the worktree must be clean and the branch merged) and only then kills the
session, removes the worktree, and deletes the merged branch; when the gate fails it
reports exactly what blocks it and touches nothing (`--abandon` overrides,
`--keep-branch` keeps the branch). Every step reports its outcome: one that FAILS is
named with git's own reason under `⚠ but these steps failed`, and the command exits
non-zero — a branch that survived a reap is never left to be inferred from a line that
isn't printed. `--snooze [--for <dur>]` silences a reap suggestion
for a dispatch you're keeping. When a tracked dispatch looks reclaimable, a live hq
gets a `» gtmux·reap-suggest … │ gtmux reap <id>` wake — reclaim is always
suggest → approve → execute, never automatic.

## `gtmux usage` — token watch

```
● api:0.0        2.1M out · ctx 85% ·  7k/m   ⚠ ctx 85%
● web:0.0         830k out · ctx 60% · 391/m
Σ claude          2.9M out ·  7k/m · 2 sessions
```

Per-session token accounting parsed deterministically from the agent's own log
(zero LLM calls): cumulative output/input, the LIVE context footprint (the last
message's input + cache tokens, judged against an evidence-inferred window),
and a 10-minute spend rate. **Layered thresholds** per agent type in
`~/.config/gtmux/usage.json`:

```json
{"claude": {"ctxWarn": 0.8, "sessionOutWarn": 20000000,
            "typeRatePerMinWarn": 30000},
 "horizonMin": 30}
```

The evaluator also **projects** (`current + rate × horizon`) so you're warned
BEFORE a wall — `ctx→80% in ~9m` — not at it. Warnings surface as an amber
`usage_warn` on the radar row (`agents --json` / digest), in `gtmux usage`, and
as a one-per-layer `» gtmux·usage·warn …` wake into a live HQ session. `--json`
is also served as `GET /api/usage`. Claude-first (other agents' logs don't carry
usage yet); the hook evaluates on every lifecycle event — near-real-time during
tool-driven work; a long silent generation settles at its next event (P2: serve
tick).

> **Network-aware launch:** gtmux prefixes agent launches (`gtmux hq` / `adopt` /
> restore / the limits command) with a proxy when needed, so you never hand-toggle
> one across networks. `~/.config/gtmux/config.json` → `"agentProxy": "auto"`
> (default) applies `http://127.0.0.1:<agentProxyPort, 7897>` **iff that port is
> listening** (your proxy tool is running — the home-VPN case) and nothing
> otherwise (intranet); an explicit URL forces it, `"off"` disables.

## `gtmux events` — the session event stream (subscription)

```
22:50:40  working          api:0.0        Claude Code (%7)
22:51:02  waiting·permission  api:0.0     Claude Code (%7)
22:53:19  idle             web:1.0        Codex (%11)
```

The hook appends every session's lifecycle event (start / finish / waiting /
background) to a ROTATED log (`~/.local/share/gtmux/events.jsonl`, active 20 MB +
1 rotated ≈ 40 MB ceiling, `eventsCapMB` config; `0` disables). `gtmux events`
prints the last hour; `--since 10m|2h` a window; `--follow` streams live and is
rotation-aware (never silently stops). `--since-seq N` is the one-shot DELTA read
(everything strictly after sequence N, oldest first, combinable with
`--severity`/`--json`) — the pull-on-wake primitive: gtmux HQ is woken by a
signal line naming a sequence range and pulls exactly that delta, on any agent
that can run a CLI command (no background tail required). This is the
terminal-native SUBSCRIPTION to the same events the apps get over SSE.

An unfiltered `--since-seq` read **run from the hq home** is also hq's consumption
writeback: it advances the watermark to the end of what it returned, which is what stops
the `unread` knock (see [The watermark](#the-watermark--why-nothing-goes-missing)).
`--ack N` writes the watermark back explicitly — for when the stream was reconciled some
other way, e.g. a full `gtmux digest`. Both are hq-only (cwd-keyed); a worker running
`gtmux events` in a repo changes nothing.

Because the rule is the exact cwd, a read from a **subdirectory** of the hq home
(`notes/`, `knowledge/` — where hq lands after writing its board) does not count. That
used to be silent, so hq read the delta, believed it had consumed, and watched the same
cursor re-knock; it now warns on stderr, naming the home to run from, while stdout and the
exit code stay exactly the same. A read from an unrelated directory stays silent — it owns
no watermark to miss.

hq's own delta pull is also **scoped to the debt**: it omits the records that never counted
(hq's own pane's lines, pane-less blinks, and gtmux's `gtmux:audit:*` trail) and says on
stderr how many it withheld.
`--all` restores the raw view; both forms consume, because neither shows hq *less* than it
owes — which is exactly what disqualifies a `--severity` read. Anyone else's read is
unchanged.

`--severity <tier>` filters to that tier **and above**, and the tiers rank
*urgency*, not relevance — so they are three different reads, not one:
the unfiltered `--since-seq` delta is what you **reconcile** with;
`--severity notable` is the **fleet-change** stream (an instruction reaching a
session — `origin:"instruction"` — plus turn-ends and lifecycle);
`--severity important` is the **escalation** subset (blocked · asking · crashed),
to triage first. A filter is a triage shortcut, never the whole picture.

The stream also carries gtmux's own **control records** — the periodic maintenance
triggers it raises for hq — rendered as `[CONTROL <event>]` with their reason:

```
09:57:16  [CONTROL gtmux:self-check]  due (daily) — review feed/ledger/memory health…
04:33:49  [CONTROL gtmux:distill]     due (weekly) — distil the period into the KB…
```

That makes "did the periodic pass actually run?" answerable directly —
`gtmux events --since 30d | grep distill`. It is worth knowing because both passes are
silent by design: without a record, an hq that never distilled looks exactly like one that
had nothing to distil. `gtmux doctor` renders the same answer as a verdict — its **HQ
maintenance** rows show when each pass last ran and flag one that has slipped past its
cadence (shown only on a machine that has an hq home).

## `gtmux resource` — local machine resource watch

```
disk 40GB free · mem 38% free (warn) · load 0.64×14 cores · power 74% (battery 2:13)   ⚠ disk 40GB free
per-agent (RSS · CPU):
  %26    252MB · 9.2%
reclaim candidates (orphans no live agent owns):
  pid 3015  100MB · 0.0%  iOS Simulator runtime (12 procs) [simulator]
    ↳ leftover iOS Simulator runtime — `xcrun simctl shutdown all`
```

Disk (`df`), memory (`memory_pressure -Q` free % + the kernel `kern.memorystatus_vm_pressure_level`
normal/warn/critical tier), CPU (loadavg÷cores), and **power/battery** (`pmset -g batt`:
charge % · on-AC vs draining · time left; absent on a battery-less host — a LOW charge
counts toward the warn/tier ONLY while draining, never on AC). **Per-agent RSS/CPU** by
walking each pane's process tree (isomorphic to token accounting), and **reclaim
candidates** — heavy processes no live pane owns, named with pid + how to reclaim
(a leftover iOS Simulator runtime aggregates into one entry; dev servers/tmux
strays surface individually). Thresholds in `~/.config/gtmux/config.json`'s
`resource` object (diskAmberGB 50 / diskRedGB 15 / loadAmber 1.0 / loadRed 1.5 /
orphanRssMB 300 / batteryAmberPct 20 / batteryRedPct 10). A resource block rides `GET /api/usage`; the serve tick emits a
`resource·warn` nudge to HQ (single-writer — one per crossing); `gtmux hq`/`new`
warn at a red line before adding load.

The **warning** is damped three ways so a value sitting on a threshold can't
re-alert (the readout stays raw — `gtmux resource` always reports what it measured):

| key | default | what it does |
|---|---|---|
| `diskHysteresisGB` | 2 | GB of headroom above the entry line before a disk tier clears (red at <15 GB clears at ≥17) |
| `loadHysteresis` | 0.15 | load÷cores below the entry line before a load tier clears (amber at ≥1.0 clears below 0.85) |
| `batteryHysteresisPct` | 3 | % of charge above the entry line before a battery tier clears (amber at <20% clears at ≥23%) |
| `confirmSamples` | 3 | consecutive agreeing samples before a tier change is believed |
| `minRestateMinutes` | 30 | quiet period before the same tier warns again — an escalation to a worse tier is exempt and always warns |

## `gtmux limits` — real subscription-window remaining

```
● session               16% used   resets Jul 13 at 1:29am
● week (all models)     60% used   resets Jul 17 at 10:59pm
● week (fable)          90% used   resets Jul 17 at 10:59pm
⚠ near the weekly cap: week (fable) 90%
```

The one number local estimation can't give you: **how much of your plan is
left**. gtmux gets it from the agent's OWN `/usage` command run headlessly
(`claude -p "/usage"`) — real server data, the user's sanctioned command, not a
reverse-engineered endpoint. Because that spawns a process, results are **cached**
(`state/limits.json`) with a 15-minute TTL, shortened to 5 minutes once any
window is near its cap; `--refresh` forces one. Configure in
`~/.config/gtmux/usage.json`:

```json
{"limitsCommand": "claude -p /usage", "limitsTTLMin": 15,
 "limitsTTLNearMin": 5, "limitsNearPct": 70, "limitsWarnPct": 85}
```

Set `limitsCommand` with an env prefix if your network needs it
(`"HTTPS_PROXY=… claude -p /usage"`), or `""` to disable. A weekly window at/over
`limitsWarnPct` marks amber and wakes a live HQ once (`» gtmux·limits·warn …`).
The `limits` block also rides `gtmux usage` and `GET /api/usage`.

## `gtmux awake` — keep working with the lid closed

```
gtmux awake on       # asks for your admin password once, then verifies it took effect
gtmux awake          # awake = on (clamshell) · up 2h13m · power battery 74%
gtmux awake off      # no password — immediate
```

Closing a MacBook's lid sleeps the system, which drops the tunnel and freezes every
agent mid-turn — so "command your Mac from your phone" only worked while the lid stayed
open. This is the switch that changes that. (It shipped as `gtmux server-mode` in
v0.44.0; the old name still works. The *feature* is still called server mode — the
command is just shorter.)

**Turning it on costs one password. Turning it off costs nothing.** That asymmetry is
the whole design: escalation is local, interactive and deliberately visible for as long
as it lasts; de-escalation is free, automatic and always possible — including when gtmux
is dead, which is exactly when it matters most.

What makes that work is a small root-owned **guard** installed in the same authorization.
It contains no code path that disables sleep — its only power is to give it back — and it
restores sleep and deletes itself when any of these happens:

| trigger | what it means |
|---|---|
| you turn it off | an unprivileged marker; the guard wakes on it within about a second |
| charge reaches 20% | you are warned at 30%, on the Mac and on your phone |
| gtmux stops running | crash, force-quit, `brew uninstall` — nothing needs to survive |
| a reboot with nobody logging in | after a startup grace, so a normal restart does not kill your session |

**It never expires.** It runs until you turn it off. Instead of a timer, the menu-bar
icon carries a slowly pulsing red dot the whole time — the same visual language as a
screen recording, because the risk this feature actually has is being forgotten.

**Battery is a supported case, not a hazard**: carrying a closed laptop between rooms
keeps working (measured — unplugged, lid shut, zero sleeps). What ends it is *remaining
charge*, not losing the adapter.

**gtmux reverts only what gtmux set.** A `disablesleep` it did not stamp is reported with
the manual undo command and never changed for you — the same report-only discipline
`gtmux reap` applies to an unclean worktree. `gtmux doctor` surfaces the same finding, and
stays silent on machines that have never touched the setting.

**Where to read the state** — this is the subtle part, and getting it wrong is the
feature's worst failure (announcing "sleep restored" on a Mac that cannot sleep):

| source | use it? |
|---|---|
| `pmset -g` / `-g custom` / `-g live` | ❌ never reports `disablesleep`, in either state |
| the power-management plist | ⚠️ lags a write; answers "would it survive a reboot" |
| `ioreg -r -c IOPMrootDomain` → `SleepDisabled` | ✅ the live, unprivileged truth |

`gtmux awake --json` reports both readings plus `owned_by_gtmux`, `guard`, and a
`platform` verdict. On a macOS the project has not verified, `on` says so plainly rather
than failing quietly; where the mechanism is absent it refuses **before** asking for a
password.

**Two boundaries, stated rather than papered over:**

- `gtmux serve` is a per-user LaunchAgent, so after a reboot it starts only once someone
  logs in. On a FileVault Mac with nobody there the heartbeat never resumes and sleep is
  restored — the right fail-safe, but it means server mode does **not** survive an
  unattended reboot. gtmux will not "fix" that by touching FileVault or auto-login.
- The underlying setting is **undocumented by Apple**. It is verified on macOS 26 and
  detected at runtime; if a future macOS drops it, `on` refuses with a reason instead of
  silently doing nothing.

Your phone can **see** this state (a ring on the connection dot, and a row in Servers /
Manage Mac) but never change it — every path to changing it ends at a password typed at
the Mac, so a remote switch that could only turn it off would send you back to the laptop
anyway.

## `gtmux restore`

Quitting your terminal leaves the tmux server and all sessions alive — only the
tabs are gone. After reopening, run **once** in any tab:

```sh
gtmux restore            # one terminal tab per tmux session, all attached
gtmux restore --pick     # choose which sessions: "1 3" / "1,3", Enter = all, q = cancel
gtmux restore --one      # attach the next unattached session in this tab
gtmux restore <name>     # attach a specific session here
gtmux restore --dry-run  # print what would happen, change nothing
gtmux restore --plan     # preview: which sessions + agent conversations would come back (read-only)
gtmux restore --plan --json   # the same plan as JSON (the menu bar's source for its expandable restore row)
```

A real `gtmux restore` prints this same plan up front — the sessions it's about to
bring back and the agent conversation (goal) under each pane — so you see what's
being restored as it happens and have a review checklist afterward. `--plan` is that
preview on its own: it reads the last resurrect save + the resume records and starts
no tmux (safe to run or poll anytime). An agent line marked `×` is a conversation
whose transcript is gone from disk and won't resume.

**Which panes get an agent back.** Only the ones that were actually RUNNING one when
the layout was saved — restore reads that from the save's own record of each pane's
command, not from "an agent lived here once". A pane that was a plain shell at save
time comes back a plain shell, even if you ran an agent in it last week. (It used to
come back with `claude --resume …` typed into it: the resume records are written by
the agent's hooks and never pruned, so every pane that had ever hosted a conversation
was a permanent target — one reboot turned 10 live agent panes into 16.) Which
conversation a live pane gets is still the resume record for that pane; if the record
is missing, restore reads the id out of the `--resume` the save recorded it running.

Diagnosing it needs no reboot — point `XDG_DATA_HOME` at a copy of any save and
preview it read-only:

```sh
mkdir -p /tmp/probe/tmux/resurrect && cd /tmp/probe/tmux/resurrect
cp ~/.local/share/tmux/resurrect/tmux_resurrect_<stamp>.txt . && ln -sf tmux_resurrect_<stamp>.txt last
XDG_DATA_HOME=/tmp/probe gtmux restore --plan     # what restore would bring back from THAT save
```

The first run pops an Automation permission dialog ("wants to control Ghostty" —
or iTerm2/Warp, whichever hosts your tabs); click Allow. **After a reboot** the tmux server is gone too; `gtmux restore`
starts tmux and explicitly drives
[tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) to restore the
last autosave (it waits for the restore to finish — large layouts take 30s+ —
and if a saved layout exists but can't be restored it refuses to overwrite it).
Running programs are not restarted — relaunch e.g. with `claude --resume`.

Each pane's previous output (scrollback) comes back too — a snapshot — when
resurrect is set to capture it. Recommended in `tmux.conf`:

```tmux
set -g @resurrect-capture-pane-contents 'on'   # snapshot each pane's scrollback
set -g history-limit 50000                     # how much scrollback to keep/restore
```

> The shell's **↑ command history** is separate — it lives in your shell's
> histfile, not in resurrect. By default it's written only on shell exit, so a
> reboot loses recent commands. To persist it immediately (bash):
> `shopt -s histappend; PROMPT_COMMAND='history -a'` in `~/.bashrc` (zsh:
> `setopt INC_APPEND_HISTORY`).

## `gtmux overview`

```
gtmux overview — 2 sessions · 3 windows · 5 panes

▶ web-api              1 window · 1 pane
    0: web-api *  (1 pane)
● worker               2 windows · 4 panes
    0: editor  (1 pane)
    1: claude *  (3 panes)

▶ current  ● attached  ○ detached   * active  Z zoomed  • new output
```

A sessions/windows/panes summary from any shell. `--popup` is size-fitted for a
tmux `display-popup`, so you can bind it to a key and float it over a full-screen
program without interrupting it.

## `gtmux focus`

```sh
gtmux focus web          # bring the terminal tab showing session "web" to front
gtmux focus %11          # jump to that exact window+pane, then focus its tab
```

Each tab title is `session — window`, so `focus` finds the matching tab and
brings it to front (via the terminal's AppleScript). A pane id (`%N`) also
selects that window+pane inside the session, so you land exactly where the agent
is — which is how a notification click drops you on the agent that just finished.

**A session with no window open** — a `--headless` spawn, or one you detached — has no tab
to bring forward, so `focus` OPENS one and attaches it. It used to select the pane inside
tmux and then search for a tab that could not exist, which looked exactly like a broken
jump. The test is the client count (`session_attached`), not how the session was started:
a headless session someone attached later is an ordinary jump. Surfaces mark such a row
(`no window` / `无窗口`) so you know a tab will open before you click.

> Needs `set-titles on` with `set-titles-string '#S — #W'` so tab titles stay in
> the format `focus` matches. If another tool also writes the tab title, disable
> that so titles stay authoritative.

Host terminals: **Ghostty** and **iTerm2** are fully driven (exact tab via
AppleScript). **Warp** is best-effort — it has no AppleScript dictionary, so
`focus` jumps to the exact tab only when that tab's Warp session uuid was
recorded into the tmux session env (gtmux's own restore/new attach does this;
or add `WARP_TERMINAL_SESSION_UUID` to tmux's `update-environment` to cover
hand-opened tabs), and otherwise just activates the Warp app; `restore`/`new`
open Warp tabs via launch configurations. Other terminals fall back to the
Ghostty driver. The host is auto-detected; `GTMUX_TERMINAL=ghostty|iterm2|warp`
overrides the detection.

## `gtmux attach` — work in a remote session from another machine's terminal

Where `focus` jumps to a LOCAL tab, `attach` opens a REMOTE pane in your current
terminal (Ghostty / iTerm2 / Terminal) as a raw, interactive passthrough — the local
terminal becomes the remote tmux session, over the same `gtmux serve` surface (a
WebSocket, `GET /api/attach`), honoring the owner/guest token scope.

```sh
# owner — full access with the serve token:
gtmux attach http://<mac>:8765 --token <serve-token> %12

# guest — a scope-restricted share link (from `gtmux share new`, or the menu bar's
# Sharing → New link); attach exactly what the host allowed:
gtmux attach 'https://<mac>.example/#g=<token>' %12

gtmux attach <target>            # omit the pane: auto-attach the only one, else pick
gtmux attach <target> --read-only  # watch only, never send input
gtmux attach <target> --predict    # experimental: hide round-trip lag while typing
```

**`--predict` (experimental, off by default)** — predictive local echo, the mosh idea
adapted to our WS/TCP bridge. Over a slow link every keystroke otherwise waits a full
round-trip to echo (~340 ms on a cross-continent tunnel). With `--predict`, your own
printable typing and backspaces appear **immediately, underlined** to mark them
unconfirmed, and are erased the instant authoritative output arrives — **the server screen
always wins**, so a wrong guess is corrected within one round-trip rather than left
standing. The real keystroke is forwarded to the pane unchanged; prediction only paints
locally. It is deliberately conservative: **adaptive** (nothing is drawn on a fast/LAN
link, where the echo already feels instant), never inside a **full-screen TUI** (the
server tells the client when the pane is on the alternate screen), and any state-changing
key (Enter, ESC, arrows, Ctrl-C, Tab) ends the prediction epoch rather than guessing
across an unpredictable screen change. The client learns the cursor from the server (tmux
knows it) rather than emulating a terminal — see
`docs/design/mosh-predictive-echo-research.md`.

Omit `%N` and, when several panes are attachable, `attach` shows a **numbered menu**
(session · agent · status · task per row) and connects to the one you pick — Enter takes
the first row, `q` cancels. Piped or scripted (stdin not a TTY), it instead prints the
list and exits non-zero, so automation never blocks on the prompt.

- **`<target>`** is a host (+ `--token`, → **owner**, full) or a `…/#g=<token>` share
  link (→ **guest**, restricted to the host's view/input allowlists — a view-only pane
  is read-only, and a non-viewable pane is refused).
- **`%N`** (optional) is the tmux pane id to attach — it selects the SESSION that pane
  is in. Omit it to auto-attach when there's a single session, or (on a TTY) pick from a
  numbered menu of the choices; non-interactively it lists them and exits non-zero.
- **Detach** with tmux's own `<prefix> d`, or **`Ctrl-]`** (the local escape hatch).
- Scope is enforced server-side; `--read-only` is a convenience, not the security
  boundary. See `docs/design/remote-attach-research.md` for the design + trade-offs
  (raw passthrough over WS/TCP, flow control, resize, latency).

> Needs the host reachable: on the LAN directly, or from anywhere via `gtmux tunnel`
> (the WebSocket rides the same tunnel as the radar). The guest side is set up entirely
> in the menu bar (per-pane 👁 See / ⌨️ Type + New link) or with `gtmux share`.

## `gtmux pair` — enroll your own devices (full control)

```
gtmux pair                  # mint ONE one-time code, printed three ways:
                            #   phone QR · browser https://…/#c=<code> · a one-line
                            #   `gtmux attach '<url>/#c=<code>'` for another terminal
gtmux pair list             # your paired devices (guests live under `gtmux share`)
gtmux pair revoke <id>      # cut one device off, effective immediately
```

PAIR is the owner track of the pair/share model: the enrolled device is **you** —
full view + input on every session, same as sitting at the Mac. All three media
redeem the same short-lived code (5 min, single use) into the same revocable
roster. The terminal medium persists its token in `~/.config/gtmux/remotes.json`
(0600), so afterwards a bare `gtmux attach <host>` just works; `pair revoke`
invalidates it instantly. The link carries the tunnel URL when `gtmux tunnel` is
up, else a LAN address. `gtmux devices` remains as an alias of the roster.

### `gtmux devices --push` / `--forget-push` — inspect + clean up push tokens

```
gtmux devices --push                       # roster annotated with each device's push
                                           #   token (✓ env·kinds) + any UNLINKED tokens
gtmux devices --forget-push <id|orphans|all>  # drop push tokens (host-only)
```

Push tokens are bound to the enrolled device that registered them, so
`gtmux devices revoke <id>` already stops that device's notifications. `--push`
shows the binding; `--forget-push` clears tokens by selector — a device `id`,
`orphans` (only UNLINKED legacy tokens, from before device-binding), or `all`. Use
`orphans` to clear a stale token stranded from an old app that never unregistered
(the "a removed phone keeps getting notifications" case). Host-only (the local
master token); a remote device/guest is refused.

## `gtmux share` — scoped, revocable access for a collaborator

```
gtmux share new --label Alice --view %1,%2 --type %1 --expires 24h
gtmux share set a1b2c3d4 --type %2            # edit ONE link (omitted flags untouched)
gtmux share link a1b2c3d4 [--json]            # re-show an existing link's URL (+ QR)
gtmux share on|off                            # consent master switch for ALL guest typing
gtmux share status [--json]                   # per-link scope summaries
gtmux share revoke a1b2c3d4
```

SHARE is the collaborator track of the pair/share model (PAIR = your own devices,
full control; SHARE = a guest link, least privilege). **Each link carries its own
scope**: which panes the guest may SEE (`--view`) and TYPE into (`--type` ⊆ view),
plus an optional expiry (`--expires 45m|24h|7d`, default never — expired links fail
auth like revoked ones). Typing additionally needs the host consent (`share on`,
default off). Everything is enforced server-side; the web page / app only mirror it.

A link minted without `--view/--type` copies the current global lists (the
template). The legacy global forms (`share add/remove`, `share view add/remove/
clear`) still work but FAN OUT to every existing link — per-link tailoring should
use `share set`. `status --json` carries each guest's `view_panes`/`panes`/
`expires_at` and never a bare token. It also reports **who has used the link** —
`last_seen`, `platform` (`Chrome 141 · macOS`), `last_ip` — all absent until someone
has, which is itself the answer. A link is a standing grant handed to someone else,
so "has anyone walked through it, and from where" is the question worth answering;
what it PERMITS is already on the row above. A link's URL is printed at mint time; to copy
it again later use `gtmux share link <id>` (or the menu-bar row's copy button) —
it re-hands the same `#g=` URL (full-scope callers only; guests can't).

## `gtmux whatsnew` — what changed for YOU

```sh
gtmux whatsnew                 # everything newer than the version you're running
gtmux whatsnew --since v0.36.0 # from a specific version
gtmux whatsnew --all           # every release we have notes for
```

Per release, the lines written **for users** — not commit subjects. `gtmux update` prints
the first few of these automatically after installing; this is the full list.

The source is a `user:` block in the release's **tag message**, which goreleaser copies
into the release body. An optional `user-zh:` twin carries the same notes in Chinese:

```
git tag -a v0.40.0 -m "v0.40.0 — …

user:
- spawn --title now names the session
- restore returns you to the window you were on

user-zh:
- spawn --title 现在会命名会话
- restore 会把你带回原来的窗口
"
```

Both `gtmux update` and `gtmux whatsnew` print the block matching your language
(`GTMUX_LANG`): zh prefers `user-zh:`, en prefers `user:`, and either falls back to
the other when a tag carries only one — so older single-block tags keep working,
whichever language they were written in. The blocks may appear in either order; each
ends at a blank line, a heading, or the other block's marker.

A release with no `user:` block contributes nothing — deliberately. A version where
nothing changed for users should say nothing, rather than paraphrasing a commit subject
into developer vocabulary.

## `gtmux config` — the few settings that are not per-run flags

```
gtmux config agent-proxy [<url>|off]   # proxy applied when gtmux LAUNCHES an agent
gtmux config tab-alert  [on|off]       # mark the terminal TAB of a session that needs you
```

Both print their current value when called with no argument.

### `tab-alert` — find the right tab without opening nine of them

With one tmux session per terminal tab, every tab is titled alike and the only way to
learn which agent is blocked on you is to visit each one. `tab-alert on` puts a **●** in
front of the tab title of any session that has an agent waiting.

- **Only `waiting` marks.** `working` and `idle` never do. A marker that appears on most
  tabs most of the time is one nobody reads — the same reason red is reserved for
  decisions.
- **Terminal-agnostic.** tmux renders the title and pushes it to the terminal, which just
  displays it, so this works on Ghostty, iTerm2, Warp, Apple Terminal alike. It is a
  GLYPH, not a colour: a coloured tab is a per-terminal capability and would not travel.
- **Your title format is kept.** Enabling reads your `set-titles-string`, stores it, and
  prepends only its own field; `off` restores exactly what it found. If you have edited
  the format since, `off` REFUSES rather than overwrite your edit with a stale snapshot —
  delete the leading `#{@gtmux_alert}` yourself in that case.
- **Driven by the agents' own hook events**, not by reading screens: a waiting marker
  lands the instant the agent reports it, and the serve tick reconciles as a backstop.
  The supervisor is not in this loop — it is a mechanical projection of state gtmux
  already has, not a judgment.
- Also switchable in the menu-bar app's **Preferences → Notifications**.

### `hqWake` — tuning the supervisor's wake channel

Hand-edited keys under `"hqWake"` in `~/.config/gtmux/config.json` (all optional; an
absent or invalid key keeps its default). What each one governs is explained where the
behavior lives — [the wake channel](#the-wake-channel--how-hq-learns-things),
[the watermark](#the-watermark--why-nothing-goes-missing), and
[self-rotation](#self-rotation--when-hqs-own-session-is-the-problem):

| key | default | governs |
| --- | --- | --- |
| `done` | `"unattended"` | done-wake mode: `unattended` \| `always` \| `tick` |
| `paneMinGapSec` | 120 | per-pane merge window for `done` wakes (seconds) |
| `tickMinutes` | 10 | summary-tick minimum interval |
| `tickBurst` | 5 | outcome count that fires the tick early |
| `unreadDebounceSec` | 120 | how long unconsumed events must stand before an `unread` knock |
| `unreadRepeatSec` | 300 | `unread` re-knock interval while the watermark stays put |
| `selfRotateCtx` | 0.75 | context-fraction breach line (0 disables) |
| `selfRotateHours` | 12 | session-age breach line (0 disables) |
| `selfRotateTurns` | 300 | hq-turn-count breach line (0 disables) |
| `selfRotateRepeatSec` | 1800 | re-knock pacing while the breach stands |
| `selfRotateFloorSec` | 43200 | longest an unchanged breach may stay silent |
| `selfRotateCheckSec` | 300 | how often the self-rotate sensor evaluates |

## tmux integration

gtmux is just a CLI — bind whatever keys you like in `tmux.conf`. Suggested:

```tmux
set -g set-titles on
set -g set-titles-string '#S — #W'
bind g run-shell -b "gtmux overview --popup"
bind a display-popup -E -w 80% -h 60% "gtmux agents --watch --popup"
bind J run-shell "gtmux focus --last"
```

### Put the pane ids in your tab titles (optional)

gtmux names a pane by its tmux id — `%23` — on every surface, and `gtmux focus %23`
takes that id directly. Two lines make the id visible on the OTHER side too, so a row
in the app and a tab in your terminal name the same thing:

```tmux
set -g automatic-rename-format '#{b:pane_current_path} #{P:#{pane_id} }'
set-hook -g pane-exited 'set-window-option automatic-rename off ; set-window-option automatic-rename on'
```

The window name then lists EVERY pane in that window (`gtmux %23 %24`), and since
`set-titles-string` is `#S — #W` the tab inherits it — no change to the title format,
so `focus`'s tab matching is untouched.

- **`#{P:…}` iterates the window's panes**, so this does not follow focus. An
  active-pane id was the first design and measurement killed it: tmux re-evaluates the
  format on its own schedule, so the name lagged the real active pane — a stale pointer
  dressed as a live one.
- **The hook is required, not decoration.** Adding a pane re-evaluates the name
  immediately; closing one does NOT. Without the hook your tab keeps advertising a pane
  that is gone, which is worse than showing no ids at all.
- **Inside a split window**, if you want each pane to wear its id on screen as well:

  ```tmux
  set -g pane-border-status top
  set -g pane-border-format ' #{pane_id} #{pane_current_command} '
  ```

  `doctor` does NOT suggest this one. `pane-border-status` is `off` by default, so turning
  it on costs a permanent screen row per pane in every split — a real price for something
  the window name already carries. Your call, not a recommendation.
### Give a plain pane a title worth reading (optional)

An agent writes its own pane title — that is why an agent row reads `提炼本周研发周报质量部分汇总`
while the shell pane beside it reads `bash`. A shell writes nothing, so gtmux falls back to the
command name, and every plain row looks the same.

Two shell hooks fix it: the title becomes the command while it runs, and the directory at the
prompt.

```bash
# bash — in the file your LOGIN shell reads (see below)
if [ -n "$TMUX" ]; then
  trap 'printf "\033]2;%s\007" "$BASH_COMMAND"' DEBUG
  PROMPT_COMMAND='printf "\033]2;%s\007" "${PWD##*/}"'"${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
fi
```

```zsh
# zsh — ~/.zshrc
if [ -n "$TMUX" ]; then
  autoload -Uz add-zsh-hook
  gtmux_pane_title_preexec() { print -rn -- $'\e]2;'"$1"$'\a' }
  gtmux_pane_title_precmd()  { print -rn -- $'\e]2;'"${PWD:t}"$'\a' }
  add-zsh-hook preexec gtmux_pane_title_preexec
  add-zsh-hook precmd  gtmux_pane_title_precmd
fi
```

- **Not `~/.bashrc`.** A tmux pane runs a LOGIN shell, and a login bash reads
  `.bash_profile` / `.bash_login` / `.profile` — the first that exists — and never `.bashrc`.
  Putting it in `.bashrc`, which is where most recipes put it, does nothing in tmux.
  `doctor --fix` picks the file your shell actually reads, and only ever appends to one that
  already exists (creating `.bash_profile` where you keep `.profile` would shadow it).
- **`add-zsh-hook`, not a bare `preexec()`.** Defining that function replaces whatever you
  (or oh-my-zsh) already had.
- **Gated on `$TMUX`.** Outside tmux this would fight your terminal's own tab title, which
  gtmux matches on to jump.
- **`gtmux doctor` measures the result, not the config**: it counts how many plain panes have
  a title that says anything, because a hook can be written a hundred ways and only the
  outcome matters. New shells only — panes already open keep their titles.

- **`gtmux doctor` reports this row and `--fix` offers it** — as a suggestion you own,
  never a silent write. If you already have your own `automatic-rename-format`, `--fix`
  APPENDS the ids to it rather than replacing it. gtmux does not rename your windows:
  `rename-window` would turn `automatic-rename` off for that window and overwrite your
  format for good.

## Notification hook

`⏸ waiting`, `✓ latest`, and click-to-jump notifications rely on a hook writing
state files under `~/.local/share/gtmux/`. gtmux ships that hook built in — no
external script needed:

```sh
gtmux install                       # asks: hooks | app | all
gtmux install hooks                 # Claude, one-time setup (macOS)
gtmux install hooks --agent codex   # or cursor|gemini|copilot|kiro|opencode
gtmux uninstall [hooks|app|all]     # reverse it (asks when no target)
```

`install-hooks` registers `gtmux hook` in `~/.claude/settings.json` on the
`Stop`, `Notification`, and `UserPromptSubmit` events (idempotent; preserves
other hooks and backs the file up). `gtmux hook` is the producer — Claude Code
runs it, you don't — and writes state purely by event **timing**, telling a
permission request from an idle nudge without reading message text.

**Other agents:** `--agent codex|cursor|gemini|copilot|kiro|opencode` wires that
agent's own hooks file instead. **Codex** uses its additive hooks system
(`~/.codex/hooks.json` + `features.hooks`), so it **coexists with any existing
`notify`** (e.g. computer-use) rather than replacing it. **opencode** has no
command-hook file, so gtmux installs a small JS plugin
(`~/.config/opencode/plugin/gtmux.js`) that forwards its events. `gtmux doctor --fix`
offers to wire whatever agents it detects.

Notifications are delivered by the menu-bar app — no `terminal-notifier` needed.
The hook queues a request under `~/.local/share/gtmux/notify/` and `Gtmux.app`
posts a native banner (shown as **Gtmux**, with the agent icon and a **Jump**
action; *finished* is calm and silent, *needs your input* sounds). Clicking it
lands you on the exact pane. Grant "Allow Notifications" on first run and keep
the app running to receive them.

## Permissions

gtmux asks for only what it needs:

- **Automation (control your terminal — Ghostty / iTerm2 / Warp)** — required for
  `focus` / `restore` / `new` and notification click-to-jump. macOS prompts the
  first time gtmux drives the terminal via AppleScript; click **Allow**.
- **Notifications** — so the menu-bar app can post agent banners. Allow on first run.
- **Launch at login** *(optional)* — only if you enable it in Preferences.

It does **not** need these — if macOS prompts, you can safely **Deny** with no
loss of function:

- **App Management ("modify apps on your Mac")** — gtmux never modifies other
  apps; its code only ever touches its own bundle (on update/uninstall). The
  prompt can appear when macOS attributes *another* app's self-update to gtmux's
  long-running background process via its responsible-process chain. Denying
  changes nothing for gtmux.
- **Files & Folders (Downloads / Desktop / Documents)** — gtmux doesn't read
  these. The prompt can appear when `restore` recreates a tmux session whose
  working directory lives in one of them — that's `tmux` (run by gtmux) opening
  the folder. Safe to deny.
