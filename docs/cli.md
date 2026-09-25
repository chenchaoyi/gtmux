# CLI & commands

**English** · [中文](cli.zh.md)

| command | what it does |
| --- | --- |
| `agents [--watch\|--json]` | coding agents across your panes: who's waiting / working / idle, where, and the pane id to jump to |
| `panes [--json]` | EVERY tmux pane (not just agents), tiered agent/plain — the superset behind the pane browser |
| `overview [--popup]` | sessions / windows / panes summary; `--popup` fits a tmux popup |
| `restore [--pick\|--one\|<name>\|--dry-run\|--plan[ --json]] [--resume-agents=auto\|type\|off]` | one terminal tab per session, attach all; optionally relaunch captured agent conversations; `--plan` previews what would come back |
| `focus <name\|pane-id\|--last>` | jump to a session's tab; a pane id (`%N`) lands on that exact pane; `--last` = the most-recently-finished agent |
| `new [name]` | start a new tmux session in a fresh terminal tab |
| `adopt <session_id>…` | move a sensed non-tmux (native) agent session into tmux |
| `doctor [--fix [--yes] \| --bundle]` | health check grouped by concern; on a TTY it offers to fix improvable rows inline; `--fix` is the one-stop setup (hook, set-titles, restore, the app); `--bundle` packs a bug report |
| `install [hooks\|app\|all]` | install what gtmux needs; with no target it asks. `install hooks --agent codex\|cursor\|gemini\|copilot\|kiro\|opencode\|kimi` wires another agent |
| `uninstall [hooks\|app\|all]` | remove it again; with no target it asks (the two have very different consequences) |
| `serve [--port N]` | read-only HTTP+SSE radar for the mobile app / browser mirror (behind a VPN or tunnel) |
| `tunnel [--backend cloudflare\|self] [--quick] [--service] [--redeem <code>] [--servers] [--server <id>]` | expose the radar from anywhere — Standard (Cloudflare) or Direct (self-hosted / paid); `--servers` lists the Direct servers with the round trip from this Mac, `--server <id>` moves this Mac to one; see [phone.md](phone.md) |
| `pair [list\|revoke <id>]` | enroll YOUR OWN devices (full control): one one-time code as phone QR / browser link / a one-line `gtmux attach` |
| `share [new\|set\|link\|on\|off\|revoke <id>\|status]` | scoped, revocable links for collaborators — per-link view/type allowlists (see below) |
| `attach <host\|pair-link\|share-link> [%pane]` | bridge a remote tmux pane's PTY to your local terminal (owner or guest) over the serve WebSocket |
| `devices [revoke <id>\|--push\|--forget-push <id\|orphans\|all>]` | the paired-device roster (alias of `pair list`/`pair revoke`); `--push` inspects, `--forget-push` clears push tokens |
| `app` (alias `menubar`) | launch the menu-bar app (`Gtmux.app`) |
| `update [--check\|--cli-only]` | self-update the CLI + menu-bar app |

Bare `gtmux` prints one screen: every command you would type, grouped by what
running it does to the machine — what only reads, what moves your terminal, what
writes into panes, what opens a port, what changes this Mac. `gtmux <command>
--help` prints that one command with its flags, and each flag says what it accepts,
what it refuses, and what has to come with it. `gtmux --help --json` is the same
table as data, for something reading rather than looking: each command carries its
group, both language halves and whether it writes, and each flag carries `values`,
`max_bytes` and `requires`. `gtmux --version` prints the version.

Output language follows `--lang=en|zh`, `$GTMUX_LANG`, `gtmux config lang`, or, when
none is set, the system locale (`LC_ALL`/`LANG`: a `zh*` locale reads Chinese;
default `en`). Everything is invoked explicitly: no shell hooks, works with any
shell.

## `gtmux agents`

Lists the coding agents running in your tmux panes, sorted by urgency.

```
gtmux agents · 7 agents · 1 waiting · 2 working · 4 idle

‖ waiting  Claude Code  api:0.0                permission to run tests %7
⠿ working  Claude Code  hq:0.0                 api is waiting on you · rest normal %1
⠿ working  Claude Code  web:0.0                refactor auth middleware %11
✓ idle     Claude Code  app:0.0                wire up the dashboard %9
✓ idle     Codex        worker:0.0             add retry backoff %8  ✓ latest
✓ idle     Gemini       docs:0.0               draft the API reference %3
● running  Claude Code  infra:0.0              — %5

jump: gtmux focus <pane>   (e.g. gtmux focus %7)
```

Each row is status · agent · location · task · pane id.

- ‖ waiting (red): blocked on you for a permission or approval mid-task; sorts to the
  very top, and is the one colour on the screen that means act now.
- ⠿ working (cyan): busy, leave it alone.
- ✓ idle (green): finished its turn, your move when ready (not urgent).
- ● running (grey): a pane with no agent turn to speak of, a plain shell.

The marks are text-presentation characters on purpose. `⏸` and `✳`, which these replace,
carry emoji presentation: a terminal may draw them from a colour emoji font that ignores
the colour it was given, which put the red on the word and not on the mark beside it.
- ⚠ errored (amber): an idle session that ended on an API or tool error (for example
  `Unable to connect to API`) instead of a clean finish. It is still idle (your move),
  and the row shows the error summary. In `--json`: `error: true` plus `error_text`.

`gtmux agents --watch` is a live, auto-refreshing dashboard (built with
[bubbletea](https://github.com/charmbracelet/bubbletea)): polls about every 1.5 s,
↑/↓ select, Enter jumps to the pane, r refreshes, q quits. It groups the fleet under
NEEDS YOU · WORKING · IDLE · RUNNING, carries a `since` column, and ends on one line of
plan state: the tightest three windows, the full one in amber. Columns give way as the
terminal narrows (the agent name first, then the task) so no row ever wraps. `--json` emits the same
data for scripts and the menu-bar app.

### How detection works (not Claude-only)

- Status comes from the pane title the agent sets itself. A leading braille spinner
  (`⠋⠙⠹…`, what most agent TUIs animate) means working; Claude Code's `✳` means idle.
- Which agent is matched by foreground command (`claude`, `codex`, `gemini`, `cursor`,
  `opencode`, …) or by a name in the title.
- Extend or override via `~/.config/gtmux/agents.json`, a JSON array of
  `{"name","commands","idleGlyph"}`; your entries win over the built-ins.
- A pane is listed only if the agent process is actually running. A leftover agent
  title over a plain shell (a resurrect-restored session never relaunched) is not
  counted.
- Agents running outside tmux (a bare `codex`/`claude` in a terminal) are sensed
  read-only via the same hook and listed under Elsewhere with `source:"native"`. They
  have no pane (no jump, no reply); a resumable one can be pulled into tmux with
  `gtmux adopt <session_id>`.

`⏸ waiting` and `✓ latest` come from state files written by the
[notification hook](#notification-hook). Without it, agents never show `⏸`;
everything else still works.

## `gtmux panes`

`gtmux panes` lists every tmux pane, coding agent or not. It is the read-only producer
behind the pane browser: a session → window → pane tree, or `--json` for a structured
array. Each pane carries a `tier` of `"agent"` (a coding-agent pane, classified exactly
as the radar does) or `"plain"` (a shell, editor, dev server, log, anything else), plus
its loc, cwd, current command, title, active flag, and copy-mode flag.

```sh
gtmux panes            # session → window → pane tree; ▸ marks agent panes
gtmux panes --json     # [{pane_id, loc, session, window, pane, cwd, command, title, active, in_mode, tier, agent}]
gtmux panes watch %N   # opt a PLAIN pane onto the radar as a watched row
gtmux panes unwatch %N # remove it
gtmux panes --watched  # list watched pane ids
```

`gtmux agents --json` is a locked contract meaning "coding agents"; `panes` is the
superset for a browser that reaches any pane. `gtmux focus`, `send` and `attach` work on
any pane id, so a `tier:"plain"` pane is a first-class focus, type and attach target; it
just does not get the agent-only intelligence (digest, 1/2/3 approval, dispatch, HQ).

The tier capability matrix:

| tier | on the radar | view/capture | focus/jump | type/send | attach | intelligence (digest / 1·2·3 / dispatch / HQ) |
|---|---|---|---|---|---|---|
| agent (tmux) | auto | ✓ | ✓ | ✓ | ✓ | ✓ |
| plain tmux pane | opt-in (`panes watch`) | ✓ | ✓ | ✓ | ✓ | — |
| sensed non-tmux agent | "Elsewhere" | — | — | — | — | — (read-only) |

A plain pane appears on the agent radar only when you opt in with
`gtmux panes watch %N`, as a distinct watched row (no agent status), and it is dropped
automatically when the pane closes. Guest share scope still gates view and type on any
pane the same way.

## `gtmux digest` + `gtmux hq`: HQ, the supervisor session

`gtmux digest` prints the fleet at a glance with meaning, as a column-aligned table:

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

A one-line count by state leads, then a section per state (needs-you, working,
completed, then errored if any). Each row is status glyph · name · goal/last/ask · a
right badge (dispatch status / ask-option count / usage warning) · a relative time.
Every field is assembled deterministically (zero LLM tokens): goal is the session's last
user prompt, last is the tail of its last reply (both from the agent's own transcript),
asks are a waiting prompt's parsed options. `--json` emits the machine form (also served
as `GET /api/digest`). A session gtmux has no transcript for still renders from radar
signals alone. Each JSON row states its perception tier as `sense`: `driver` (the
agent's hook feeds its state and its transcript feeds goal/last), `partial` (the hook is
in but no structured content resolved), `screen` (capture and process inference only).

`gtmux hq` opens (or focuses, never duplicates) HQ: your coding agent running in a
dedicated tmux session at `~/.config/gtmux/hq/`, seeded once with a playbook: read
`gtmux digest --json`, judge, drill into a pane (`tmux capture-pane`) only when
warranted, drive via `gtmux send`, report to you. The playbook is `AGENTS.md` with
`CLAUDE.md` as an `@AGENTS.md` import, so HQ can be any CLI agent. It ships in your
language (`GTMUX_LANG` en/zh) and a version change regenerates it in that same language
(prior file backed up; LOCAL.md untouched); `gtmux hq --lang en|zh` is the one thing
that switches it.

On a fresh launch with no `--agent`, `gtmux hq` asks which installed hook-equipped agent
to run and remembers the pick; `gtmux hq --agent codex` or `GTMUX_HQ_AGENT` names it
outright, and a script falls back to the default without prompting. Personalize HQ in
`LOCAL.md` (same directory): priorities, reporting style, quiet hours. That file
survives every playbook upgrade; edits to the managed `AGENTS.md` are displaced to a
backup. Notes HQ keeps in that directory persist across its sessions. In the radar its
row carries `role:"supervisor"`.

Where it runs. With no flag, `gtmux hq` keeps the window HQ already has: a live HQ is
focused, a window whose HQ has quit gets it relaunched in place, and only when neither
exists does it create its own tmux session and open a tab. A window that hosted HQ once
keeps its stamp, so every `gtmux hq` from anywhere revives HQ there. To put it somewhere
else: `--pane %N` starts it in that pane (an empty shell, `cd`'d to the HQ home),
`--here` in the pane you are typing in, `--new-pane` in a fresh split of your window.
Each moves HQ's identity to the new pane, and each refuses while an HQ is running
anywhere: there is only ever one.

Starting HQ in the pane you are typing in is a special case, because gtmux is holding
that terminal: it hands the terminal to the agent directly instead of typing into itself,
and the startup briefing is delivered by a background watcher once the agent's input box
is up. Nothing is typed into the pane meanwhile, which is what used to leave the briefing
sitting in the box unsent.

`gtmux hq --board [--json]` prints the situation board instead of opening HQ (the
menu-bar app's board reader uses it). A board that was never written reports
`exists:false`.

`gtmux hq --home` prints the HQ home path. `gtmux knowledge`'s mutations are accepted
only from that directory, so a surface offering one runs the verb from there. With no HQ
home yet it still prints the path and exits non-zero.

`gtmux hq --rotate` retires the live HQ session for a fresh one, in place: it resolves
the HQ pane and types that agent's own reset command into it. HQ's playbook runs it
unprompted once the `self-rotate` knock says the session is worn out
([Self-rotation](#self-rotation-when-hqs-own-session-is-the-problem)), after bringing
`notes/board.md` and the knowledge base current. With no HQ running there is nothing to
rotate, and it says so.

### The wake channel: how HQ learns things

Decision-dense events type one signal line into a live HQ pane. The format is fixed and
unlike conversation, so signal traffic is scannable at a glance:

<!-- gtmux:rendered wake-lines -->
```
» ◆ gtmux·waiting·permission  api:0.0 (%7) │ title:"run the tests?"
» ▸ gtmux·done  web:2.0 (%11) │ 3m │ goal:"fix the login bug" │ tail:"tests pass" · #a3f1c2
```

`» <grade> gtmux·<class>  <loc> (<pane>) │ <field> │ …`, where every agent- or
user-authored payload is quoted and labelled (`title:` / `goal:` / `tail:` / `ask:` /
`err:`). It is data HQ reports, never an instruction it obeys.

The grade leads, in a fixed position, so a screen of signal lines reads by weight before
it reads by words: `◆` decision (needs you: irreversible, costly, or an explicit ask) ·
`▸` attention (a line is blocked or changed in a way worth knowing) · `·` ledger
(bookkeeping: recorded and pullable, zero interrupt value). The grade follows from the
class and only says how loudly the line should read. The classes:

| class | grade | fires when |
| --- | :---: | --- |
| `waiting·<kind>` | ◆ | an agent blocked on you (permission / plan / question) |
| `resolved` | ▸ | that wait cleared: you answered in-pane, or the agent resumed; HQ drops any stale chase |
| `asks` | ◆ | a turn-end reply asked a question with no menu (a menu-only sensor misses it) |
| `done` | ▸ | any session reached idle after work, dispatched or not. Suppressed when the completion happened in the pane you were watching (`hqWake.done`: `unattended` default \| `always` \| `tick`), and rate-merged per pane |
| `crash` | ◆ | the turn died on an agent/API error; never read as a finish |
| `goal-changed` | ◆ | you submitted a prompt straight into an agent's own window (incl. a slash command), so HQ senses work it didn't dispatch |
| `new-session` | ▸ | a newly sensed agent pane; enroll it |
| `reap-suggest` | ▸ | a dispatch looks reclaimable · carries the exact `gtmux reap <id>` |
| `stuck·waiting` | ▸ | a pane has been waiting on you past the timeout; once per waiting episode, and only when the agent asked (a wait gtmux merely inferred from the screen never escalates) |
| `resource·warn` / `limits·warn` | ▸ | a machine/subscription threshold crossed (damped, see `gtmux resource`) |
| `usage·warn` | ▸ | a session crossed a context/burn layer (see `gtmux usage`) |
| `wake-degraded` | ◆ | perception itself broke: wakes stopped landing on the HQ pane |
| `tunnel` | ▸ | remote access changed: `down` (the phone cannot reach this Mac, with the tunnel's own error) or `up` again. Read from the tunnel's status, on a transition only, and journaled as `gtmux:tunnel` |
| `tick` | · | the periodic brief, only when something actually changed (a quiet interval costs nothing) |
| `distill` | · | a knowledge-distillation pass is due (≈ weekly, sooner once ≥5 `gtmux capture` candidates are queued); HQ folds the period's lessons into its knowledge base and prunes stale |
| `self-check` | · | HQ's own housekeeping is due (≈ daily): settle stale pending entries, memory and log health |
| `unread` | · | events are sitting past HQ's consumption watermark: a count and the cursor to pull from, no importance claim |
| `self-rotate` | ◆ | HQ's own session is worn out (context / age / turns); it hands off and rotates itself, see below |

The classes that repeat until an act clears them (`self-rotate`, `unread`, the warns)
re-check themselves: a repeat whose breach set and world are unchanged is suppressed and
re-arms on any drift, with a safety floor so a standing debt is never forgotten. A queued
wake is re-sampled just before it is typed, and dropped if its premise died meanwhile.

`distill` and `self-check` are maintenance classes: they knock at the lowest priority,
never ahead of a blocked agent, and both passes are silent unless HQ actually did
something. They are the only classes raised purely on a clock; HQ has no timer of its
own, so `gtmux serve` is what makes them happen.

Everything else is pull-side: HQ wakes, then reads `gtmux events --since-seq <n>` or
`gtmux digest`. Ordinary progress turns never touch its screen.

HQ's replies to signal lines are signal lines too: one line opening with `⟣` plus a
glyph, so its pane scans the same way its inbox does. The legend:

| reply | means |
| --- | --- |
| `⟣ ✅ <pane> <judgment> → <next step>` | a completion worth knowing about |
| `⟣ ▪ noted: <one clause>` | routine outcome, recorded to the board; nothing owed |
| `⟣ 📓 captured: <topic>` | a durable lesson written into the knowledge base |
| `⟣ ⚠ <escalation>` | something needs you, per the escalation policy |
| `⟣ ◈ 简报 <time> │ <counts> │ top item` | the periodic brief (plus up to 5 indented `· ` lines) |

### The watermark: why nothing goes missing

The classes above are priority labels: they say what to read first. Whether HQ hears
about an event at all is guaranteed by a consumption watermark, gtmux's record of how far
HQ has read the event stream. When events sit past it for `hqWake.unreadDebounceSec`
(default 120 s), gtmux knocks with a count and nothing else:

<!-- gtmux:rendered unread-line -->
```
» · gtmux·unread  7 unconsumed (%21 ×4 · %13 ×2 · control) │ pull: gtmux events --since-seq 6653 --json
```

The line names what it counted, by source (`control` is gtmux's own maintenance
triggers, `native` a non-tmux agent session), most numerous first and folded to
`+N more` past three. (Why the classes alone are not enough is in
[TROUBLESHOOTING](TROUBLESHOOTING.md#the-consumption-watermark).)

The debt clears only when HQ consumes, and only two things count: an unfiltered
`gtmux events --since-seq <n>` read from the HQ home (the everyday pull-on-wake), or an
explicit `gtmux events --ack <seq>`. A `--severity`-filtered read does not count (it
showed a subset), nor does one starting ahead of the watermark (it skipped the range
between), nor does a read that detected a sequence gap (events rotated away unread): the
pull re-warns and the knock gains a `· sequence gap` marker with a rebuild hint
(`gtmux digest --json`, then the explicit `--ack`) until HQ acks over it deliberately.
Until then the knock repeats every `hqWake.unreadRepeatSec` (default 300 s), at standing
priority.

Three kinds of record are excluded from the count: HQ's own lines; a pane-less lifecycle
blink (a `SessionStart` with no pane whose `SessionEnd` follows within seconds); and
gtmux's own audit trail (`gtmux:audit:*`: each wake batch delivered or dropped and why,
every `gtmux send` settlement, reaps, the HQ session rotation chain). The blink rule
keys on the Start/End pairing, never on the empty pane alone: a native (non-tmux)
agent's turns and gtmux's non-audit `gtmux:*` triggers are pane-less too, still count,
and for them this knock is the only channel, since the class wakes all require a pane.

`gtmux events` says WHO wrote a prompt when it was not you. Two prompt submissions on one
pane look identical whether you typed the words or HQ delivered them, and the only thing
that separates them is gtmux's record of the delivery, which the supervisor's pull view
withholds as something it does not owe. So the reader is simply told: a prompt gtmux
delivered on someone else's behalf prints with `← hq` (or `← agent:%N`) at the end of its
line, and `--json` carries it as an additive `author`. A prompt with no author is yours.

The attribution is worked out at read time from the delivery trail, so nothing about what
is owed or shown changes: the trail stays out of the consumption debt and stays hidden
from the default view. `--all` is not needed for this and is unchanged. A delivery that
was refused or failed never reached the pane and attributes nothing.

HQ's own pull shows that same set; `--all` gets the raw view back, and both count as
consumption. Nothing is ever removed from the log.

`gtmux doctor`'s HQ maintenance section reports the lag (`event consumption`) and flags
an HQ that is 20+ events or 30+ minutes behind.

Delivery guards your draft and confirms itself: a line is never typed into a non-empty HQ
input box (it queues to disk and lands when the box clears), and a batch leaves the queue
only once it has been seen on screen, so a failed send retries. Delivery is therefore
at-least-once, hence the trailing `#<id>`: the same id twice is a re-send, and HQ's
playbook says to ignore it. Every outcome is journaled (`gtmux:audit:wake-delivered` with
the full batch, `gtmux:audit:wake-dropped` with the reason: evicted / unconfirmed /
superseded) and readable with `gtmux events --all`.

### Self-rotation: when HQ's own session is the problem

`self-rotate` is about HQ itself: in a long, near-full session HQ starts to read its own
output as input that came from outside, and it cannot catch that from inside. So
`gtmux serve` watches from outside and knocks:

```
» gtmux·self-rotate  ctx 82% · 14h · 380 turns │ over: ctx 82% ≥ 75% │ board+KB current → hand off → gtmux hq --rotate
```

Three facts are sensed, and any one crossing its line is a breach:

| `hqWake` key | default | what it measures |
| --- | --- | --- |
| `selfRotateCtx` | 0.75 | live context fraction, the same `ctx` `gtmux digest` reports |
| `selfRotateHours` | 12 | session age, from the transcript's first message (a `serve` restart can't reset it) |
| `selfRotateTurns` | 300 | HQ-pane prompt submissions since the session began |

Set any of them to 0 to switch that one criterion off without losing the other two.
`selfRotateRepeatSec` (default 1800 s) paces the re-knock, `selfRotateCheckSec` (default
300 s) paces the evaluation.

The debt clears only when the session actually rotates, observed as a new agent session
id; delivering the wake does not clear it. Past `hqWake.selfRotateRepeatSec` a repeat
fires only when the breach set or the fleet has changed, and `hqWake.selfRotateFloorSec`
(12 h) is the longest it can stay silent when nothing has changed at all. HQ's playbook
tells it to do three things in order, without asking you first: bring `notes/board.md`
and the knowledge base current, record the handoff, then run `gtmux hq --rotate`, which
types that agent's own reset command (`/clear`, or `/new` for codex) into the HQ pane. A
repeated `self-rotate` after a rotation means the rotation did not take. `gtmux doctor`'s
HQ conversation health row shows the same figures. (The incident behind this class is in
[TROUBLESHOOTING](TROUBLESHOOTING.md#self-rotation).)

`"hqNudge": false` in `~/.config/gtmux/config.json` disables the channel entirely (no HQ
pane, no wake, no cost). The wake only informs: gtmux never answers another agent's
prompt, never sends navigation keys into a TUI, and the default policy tells HQ to
surface decisions to you, not take them.

## `gtmux capture`: a cheap notice into HQ's knowledge base

```
gtmux capture "<one-line lesson> @<topic>"   # topic ∈ the knowledge vocabulary: six built-ins + topics hq declared
gtmux capture --list                         # show the pending-distill queue
gtmux capture --list --json                  # the same queue, with each line's dedup key
```

```
last distill: 3d ago
2 pending-distill candidate(s):
  [pitfalls] wrangler TLS-resets from the office network — retry
```

Any worker, not just HQ, can drop a one-line candidate into the pending-distill spool
(`~/.config/gtmux/hq/knowledge/.pending-distill.jsonl`). Each line carries the lesson,
its topic tag, a dedup key (`<topic>/<lesson-slug>`, so the distill pass merges same-key
candidates), and auto-collected context (the current pane, the event high-water mark, a
timestamp, and `$GTMUX_TASK_ID` if the caller is a tracked dispatch). A candidate is not
a knowledge-base entry: HQ's distill pass decides what is durable, files it under the
right topic, and prunes.

`--list` heads the queue with when it was last drained. Five queued candidates pull the
next distill forward, so a captured lesson is filed within a day instead of waiting out
the week. `--list --json` is the same queue as one array (never `null`), each line with
its topic, timestamp, provenance and dedup key, which is what
`gtmux knowledge dismiss --capture <key>` names; dismissing a key consumes every pending
line that shares it.

## `gtmux knowledge mine`: session logs into the same queue, without a model

```
gtmux knowledge mine                 # one pass: read what the agents' logs gained, queue the leads
gtmux knowledge mine --dry-run       # show what a pass would queue, write nothing
gtmux knowledge mine --since all     # widen the correction window from 30 days to the whole stock
gtmux knowledge mine --status        # the ledger: last pass, sources, emitted, recurring errors
```

```
read 14 file(s), 3.2 MB, 61 human line(s) → 3 candidate(s) queued, 0 already emitted
  [corrections] this is wrong, do it again
      ↳ after: …moved the toggle into the header and shipped it.
  [pitfalls] bash: wrangler: command not found  (×6, 3 sessions)
```

The miner runs once a day (`hqWake.mineIntervalHours`, `0` disables). It reads the
coding agents' session logs from the byte offset it stopped at last time, subtracts
everything the machine itself wrote (tool output, harness blocks, gtmux's wake lines and
`gtmux send` payloads matched against the audit journal, compaction summaries, pastes),
and keeps two shapes as leads: a correction-shaped line a person typed right after an
agent's reply, carried with the tail of that reply, and a tool error whose normalized
signature recurs across sessions, carried with its count. No model runs. HQ's distill
pass drains the leads with `knowledge add --capture` / `dismiss --capture`.

The ledger under `~/.local/share/gtmux/mine/` keeps every offset and every emitted id,
so a pass never reads or queues anything twice, and a known error keeps counting after
its lesson was filed; `--status` shows that tally. Sources: Claude Code, Codex, Kimi
Code, and the transcript gtmux writes for opencode. Claude Code's log carries tool
output, and of the other three only Codex's does, so recurring-error leads come from
Claude Code and Codex; correction leads come from all four.

## `gtmux quiet`: how much HQ is allowed to say

```
gtmux quiet on       # CRITICAL only — the quietest setting
gtmux quiet off      # the default: NORMAL and above are surfaced
gtmux quiet status   # what is in effect right now
```

HQ grades what it finds, and this is the floor for what it prints to you. Anything below
the floor is still recorded: it goes to the attention ledger (`gtmux tasks --pending`)
instead of your screen, so turning it up loses nothing but the interruption.
`GTMUX_SURFACE_TIER` / `GTMUX_QUIET` override it for one process.

One thing is never quieted: a read-time gap in the event log. That is HQ telling you it
may have missed something.

## `gtmux knowledge`: the knowledge ledger (entries with provenance)

```
gtmux knowledge add --topic pitfalls --title "wrangler TLS-resets; retry" [--body-file -] [--capture <key>[,<key>…]] [--seq-range a..b]
gtmux knowledge supersede <id> --title "…" [--body-file -]   # replaces an entry; history stays in the ledger
gtmux knowledge retire <id> --why "…"                        # prune, with a reason that survives
gtmux knowledge dismiss --capture <key>[,<key>…] --why "…"   # reject candidates WITH a trace
gtmux knowledge promote <id> --why "…" --for <hq|machine|repo:<path>|everyone>   # WHO must know it
gtmux knowledge land <id>                                  # gtmux carries it: LOCAL.md / every agent's block / the repo file
gtmux knowledge land <id> --ref "<issue url>"              # everyone: you opened the issue the brief links to
gtmux knowledge withdraw <id> --why "…"                    # the entry was right, the promotion was not
gtmux knowledge sync [--force] [--repo <path>]             # refresh the knowledge block in each agent's instruction file
gtmux knowledge carriers                                   # each agent's instruction file and whether it is in sync
gtmux knowledge lint [--json]                              # audit: orphans, broken/outdated links, near-duplicates, stale, assumed kinds, ai-voice, title-unreadable (reports, never edits)
gtmux knowledge style [--json]                             # how an entry should read: every rule with a before and an after
gtmux knowledge search "<text>" [--topic t] [--kind k]     # ask the base a question in words, in either language
gtmux knowledge neighbours <id> | --capture <key> | --text "…"   # the closest live entries; `add` shows them before writing; `capture --list` groups the pool by them
gtmux knowledge kind <id> <facts|howto|pitfalls|judgment|decisions>   # confirm or correct what an entry IS
gtmux knowledge hit <id> [--n N] [--why "…"]               # the lesson was hit again (its count is the feedback)
gtmux knowledge confirm <id>                               # a hypothesis held up
gtmux knowledge alt <id> --lang <zh|en> --title … [--body-file -]   # write or replace the entry's other-language half
gtmux knowledge add … --sensitive --confirmed "<their words>"   # the commander's own detail: stays on this machine, written after asking
gtmux knowledge sensitive <id> [--off] --confirmed "<their words>"   # mark (or unmark) an existing entry
# add/supersede also take --kind, --tags a,b, --provenance <correction|recurrence|mined|capture|self>, --hypothesis
gtmux knowledge topic <name> --desc "…"                      # declare your own topic (clients, datasets, …)
gtmux knowledge promote <id> --why "…" [--target "…"]        # charter-level → export brief
gtmux knowledge land <id> --ref "<pr/spec>"                  # close the loop when it lands
gtmux knowledge promotions [--json]                          # the pending export queue
gtmux knowledge list [--topic t] [--json]  ·  show <id>  ·  render [--check]
```

A reader's tour of all this — where it sits, how a lesson travels, what you can change —
is [docs/knowledge.md](knowledge.md).

The knowledge base's authority is an append-only ledger
(`~/.config/gtmux/hq/knowledge/.ledger.jsonl`); the topic `.md` files are rendered from
its live entries, gtmux-owned and drift-checked (`render --check` catches a hand edit;
`render` restores). Every entry carries provenance: the event seq at write, an optional
distill range, and, when it consumed a capture candidate, that candidate's
pane/seq/task and key. `add --capture <key>` consumes every pending same-key candidate
into one entry; `dismiss` removes them with a journal trace. Every mutation appends one
`gtmux:audit:knowledge` record to the event journal.

Mutations are accepted only from the HQ home (the same cwd-keyed rule as
`gtmux events --ack`); workers record candidates with `gtmux capture`. `list`/`show`
work anywhere. A second, narrower door exists for the commander: `gtmux serve` exposes
the base to an owner-authenticated client (`GET /api/hq/knowledge`, the index without
bodies; `GET /api/hq/knowledge/entry?id=`; `POST /api/hq/knowledge/act`) and accepts
exactly two verbs from it, `land` and `retire`. Both doors journal the same
`gtmux:audit:knowledge` record.

Six topics ship built in (accounts, workflows, best-practices, pitfalls, corrections,
environment); `gtmux knowledge topic <name> --desc "…"` declares your own, rendered
immediately with its description and honored by `gtmux capture`, every knowledge verb,
and the dispatch-time echo (custom topics join pitfalls/workflows there; accounts /
corrections / environment stay out of dispatch context). Names are slugs (`a-z 0-9 -`,
≤ 40 bytes); built-ins, existing topics, and the reserved directory names refuse.
Declarations are add-only for now.

Every entry has a language (`lang`) and may carry the other as an alternate half:
`add`/`supersede` take `--lang` and `--alt-lang --alt-title [--alt-body-file -]`,
`alt <id>` adds the half later. Readers get the source when its language matches, else
the alternate, else the source with a `[zh]`/`[en]` tag. `list`/`show` follow
`GTMUX_LANG` (`--lang` overrides), the files on this machine render in the base's
majority language, the `everyone` brief and its issue go out in English. `lint` counts
`monolingual` entries; gtmux never translates, HQ writes both.

A sensitive entry is the commander's own detail (an account, a personal fact, a
credential they chose to keep here). It goes in only with `--confirmed "<their own
words>"`, the record that HQ showed them the entry and they said yes. It never leaves
the machine (`promote` accepts `hq` only; `machine.md` and repo blocks skip it), and the
Mac and the phone show a lock. `lint` reports `unmarked-sensitive` for an entry that
reads like a credential without the mark. Other people's secrets still stay out: a
pointer, never the thing.

`lint` also pairs scripts with entries: a script under `knowledge/tools/` that no live
entry names is `orphan-tool`, an entry naming a `tools/<script>` that is not there is
`broken-tool`.

Every entry sits on three axes: `kind` (`facts`, `howto`, `pitfalls`, `judgment`,
`decisions`); `provenance` (`correction`, `recurrence`, `mined`, `capture`, `self`, with
a hit count that `knowledge hit` and the transcript miner keep growing); and `audience`,
set when it is promoted. `topic` stays as the id prefix and a free tag. Entries written
before the axes are mapped at read time and marked `?` until `knowledge kind <id> <kind>`
confirms; the ledger file is never rewritten. `--hypothesis` shelves an unverified lead
in its own section, never distributed, until `confirm`.

`promote <id> --why … --for <hq|machine|repo:<path>|everyone>` writes a promotion
brief under `knowledge/promotions/` carrying the lesson, the why, the audience and its
exit, and the entry's provenance. For `hq`, `machine` and `repo` gtmux carries it:
`land <id>` writes the entry where that audience reads (LOCAL.md · the canonical
`~/.config/gtmux/knowledge/machine.md` plus an index block in every agent's global
instruction file · the repository's `AGENTS.md`, not committed) and closes the loop with
that path as the ref. For `everyone` the brief holds a prefilled GitHub issue link; a
person opens it and lands with `--ref <issue url>`. `withdraw <id> --why` drops a
promotion and keeps the entry. `gtmux doctor` flags a brief past its floor (about 2
weeks); `everyone` never counts. `knowledge sync` refreshes every agent's block,
`carriers` shows each one's state, and a block someone edited by hand is never
overwritten without `--force`.

`knowledge lint` reports orphans, broken and outdated `[[links]]`, near-duplicates,
stale hypotheses and promotions, kinds still awaiting confirmation, `ai-voice` (an entry
that reads like a machine wrote it) and `title-unreadable` (a title that names a field
where it had one line to say what happens). It never edits, and
its one-line summary rides the self-check knock.

`knowledge search "<text>"` asks the base a question in words. `neighbours <id>` asks a
different one, what is near this entry, and `add` asks it before it writes. All of them go
through one retrieval, which is the only thing in the package that decides what a match is.
It tokenizes ASCII words and CJK bigrams, so a question with no spaces in it works, and it
weights a shared word by how rare it is across the base, so a word a handful of entries
carry counts for more than one most of them carry. Measured against the 904 `[[link]]`
pairs the supervisor had drawn by hand across 720 entries, that weighting takes recall from
36% to 55% at ten results. There is no model and no index: one pass to count, one to score,
about 60ms over 720 entries. `capture --list` groups the pool into families so one
`add --capture k1,k2,…` files them as one lesson.

`knowledge style` is where those rules are written down, and the lint takes its matchers
from the same table, so the two cannot disagree. Each rule says what to do, why, and shows
a sentence before and after, in English and Chinese. A rule is either checked or left to
you: the tiering follows the humanizer skill, where a tell counts in proportion to how
rarely a careful writer would make it on purpose. Chat residue, a decorative `⇒`, an
in-house coinage, a guess stated as a fact, a body that restates its title and a title
made of identifiers count on one sighting; dashes, bold runs and "not X, but Y" count only in
company; and whether a contrast is earned or a sentence carries a fact is nobody's match to
make, so those rules are stated and never checked. Code, tables and quoted spans are not
read at all. Two rules are here for the reader that is an agent rather than a person:
everything executable or checkable stays verbatim, and an entry says who, where, what
happened and what to do.

A topic file written by hand before the ledger existed is moved to
`knowledge/legacy/<topic>.md` on the first mutation touching that topic; the render
links to it and the dispatch-time echo still consults it (details in
[TROUBLESHOOTING](TROUBLESHOOTING.md#knowledge-base-migration-and-the-phone-door)).

## `gtmux spawn` / `gtmux send` / `gtmux tasks` / `gtmux reap`: verified dispatch

`gtmux spawn <goal>` dispatches new work to a coding agent and confirms it landed:

```
gtmux spawn "add a --dry-run flag to the deploy script"
gtmux spawn --title fix-auth-mw --worktree feat/dry-run --model opus "add a --dry-run flag"
gtmux spawn --pane %14 "keep going, then run the tests"
gtmux spawn --json "…"   # → {task_id, pane_id, loc, title, session, delivered, state, judged_by, evidence}
```

Anything longer than one short line goes through `--goal-file <path>` (or
`--goal-file -` for stdin); `gtmux send` has the same channel as
`--message-file <path|->`:

```
cat > /tmp/goal.txt <<'EOF'
把 `hqPlaybookVersion` 提到 13，并且：
1. 运行 `make check`
2. 别让 shell 碰 $HOME 或 `for f in *; do echo $f; done`
EOF
gtmux spawn --title bump-playbook --cwd ~/src/gtmux --goal-file /tmp/goal.txt
gtmux send %14 --message-file /tmp/reply.txt
```

A goal passed as a command-line argument is parsed by your shell first: a backticked span
is executed, `$foo` is expanded, and a newline ends the command. The file channel has no
shell on it: the bytes go file → gtmux → `tmux load-buffer -` (a pipe) → the agent's
input box, with at most one trailing newline stripped (every heredoc appends one).
Passing both a file and a positional goal is an error. The positional form is fine for
short instructions (`gtmux spawn --pane %14 "keep going"`).

Re-running a failed dispatch converges on one worktree, one session, one ledger entry:
`--worktree` reuses a worktree that already serves that branch (a path held by a
different branch is still a hard error); spawn adopts its own previous attempt (a ledger
entry that owns its session, never got its goal delivered, and still has a live pane)
and updates the same ledger row; and a worktree or branch this invocation created is
rolled back when a step fails with nothing resumable.

`--title` names the window's purpose as a concise verb-object kebab slug (`fix-auth-mw`,
`review-pr-518`), which becomes the window and pane name across tmux, the radar, and the
app. On success `spawn` reports the handle `<loc> (%pane) · <title>`; `loc` is the live
tmux locator `session:window.pane`, recomputed each read so it stays correct under
`renumber-windows`. Refer to a spawned window by that `loc %pane · title` so you can
jump by number; HQ's playbook requires a `--title` on every dispatch and this handle in
every report.

`spawn` refuses to run a worker in the HQ home (an explicit `--cwd` naming it, the
inherited cwd when `--cwd` is absent, or `--pane` reuse of a pane sitting there): the
home's `AGENTS.md` is the HQ charter, and a worker launched there would read it and
impersonate HQ. Pass `--cwd <project dir>`.

`spawn` launches the agent (a fresh detached session by default, `--pane <id>` to reuse
one, or `--worktree <branch>` for an isolated git worktree) through the network proxy
by construction, waits for the agent to come up, then delivers the task via a tmux paste
buffer and verifies it landed.

Waiting for the agent is a real gate. The pane is ready once its composer row (the
agent's input box) is drawn, no trust gate or choice menu is up, no boot banner is on
screen, and two captures are byte-identical (or the agent's session-start event has
fired). A boot banner (`Connecting…`, `Loading…`) holds the gate; a standing notice
such as `⚠ N MCP servers need authentication · run /mcp` names an action only you can
take and never clears, so it does not. On timeout the failure reads
`✗ NOT delivered → <handle> — evidence: … blocked by: <the line that said no>` followed
by the pane's bottom region.

Verification is layered. For a hook-equipped agent (Claude Code, Codex, …) the receipt
is the agent's own `UserPromptSubmit` event, no screen-scraping; otherwise a two-frame
screen read locates the input box structurally. A receipt-confirmed landing is final,
and before any `delivered:false` the receipt is checked once more so a confirmation
arriving at the deadline is not lost. The JSON result says which layer judged it
(`judged_by: driver|screen`). `driver.<agent>.receipt: false` or `driver.enable: false`
in `~/.config/gtmux/config.json` forces the screen-read path, for isolating
event-channel faults.

The paste is bracketed, so a multi-line instruction lands as one draft and the separate
Enter submits it once. The only success is a confirmed landing: a swallowed Enter is
re-sent with backoff, a fragment paste is retried, and a timeout reports
`delivered:false` with on-screen evidence. A retry never duplicates: a re-paste happens
only into a box confirmed empty (the clear key empties one line, so a multi-line draft
can survive it), and a paste that merely rendered late is left alone. A queued
submission is reported as `state:"queued"`. A re-send interlock refuses an identical
payload to the same pane within a window (so a duplicate `/compact` can't double-fire);
`--force` overrides it. Pre-flight checks (proxy, machine resource, subscription window)
are advisory and never block.

`--oneshot` dispatches a one-shot, non-interactive worker through the agent's headless
mode (`claude -p … --output-format stream-json`, `codex exec --json`); it is accepted
only for a headless-capable agent and refused otherwise, never degraded to an
interactive spawn. The goal travels as an argument, so there is nothing to paste or
land-verify; the run still lives in a tmux pane (its JSON stream visible, its radar row
present, reap applicable), and done or crash comes from that stream plus the exit code.
A one-shot pane is watch-only: you cannot take over mid-run. `--headless` only
suppresses the terminal tab; a `--headless` spawn is still a fully interactive session
you can attach to and steer.

### `gtmux send`

`gtmux send <pane> <text>` uses the same land-verification by default (it returns as
soon as it confirms); `--no-verify` opts out, `--force` overrides the interlock, and
`--json` prints the verified result (`{delivered, state, judged_by, evidence}`,
verified sends only).

**A send never writes into someone else's unsubmitted line.** A paste appends to the
input box, so delivering onto a half-typed line would submit their words with your
payload. Every path (the CLI, HQ, `--no-verify`, and the phone) reads the draft first
and refuses (`state:"refused-draft"`, nothing written) with the draft quoted back.
`--force` waives it; the phone's idempotency key does not, since a send from another
device has nobody present to undo it.

This draft guard runs only on a pane a known agent drives (a pane running vim, ssh or
some other TUI has no composer, and reading one there returns the transcript as a
"draft"). It reads the colour capture, so the agent's own dim suggested-next-command is
not mistaken for your typing, and wants the same thing in two frames before refusing.
Everything it cannot judge lets the send through:

| what it sees | what it does |
|---|---|
| no agent in the pane (no composer) | sends |
| the capture fails (tmux hiccup, pane gone) | sends |
| the pane is scrolled (copy-mode) | sends |
| a plain shell, no input box | sends |
| your own text, the same message being re-sent | sends (idempotent) |
| someone else's text, seen twice | **refuses** |

It costs at most two reads and one poll interval, and never loops.

A send that ends `failed` may be retried immediately: the interlock drops the record of
an attempt that never landed, so the retry is not answered with `refused-duplicate`. A
`queued` send keeps its record; the agent already took it. `--no-verify` and the mobile `POST /api/send` skip only the confirmation: every text
path pastes and then sends Enter as its own key, so an unverified send cannot split a
multi-line message either. `--key` remains a single keystroke. For anything longer than a
short line, use `--message-file <path|->`.

A plain terminal pane (no coding agent in the pane's process subtree, a bare shell such
as bash or zsh in the foreground) is typed into directly: text then Enter, no input-box
confirm and no re-send interlock. `--json` reports such a send as
`{"delivered":true,"state":"sent"}`. Anything not provably a plain shell (vim, ssh, an
agent the process scan missed) keeps the agent pipeline.

### `gtmux tasks`

`gtmux tasks [--json]` is the dispatch / needs-you ledger: every task you spawned with
its live status (undelivered / waiting / done / working / gone), needs-you first.
`--verbose` adds archived entries and the attention columns (tier · priority · surfaced ·
disposition).

`undelivered` leads the list. The ledger records the delivery verdict, so an entry whose
goal never reached the agent reads `✗ undelivered` however idle its pane looks. A
`queued` delivery is not undelivered (the agent accepted it, behind the current turn). A
`gtmux send` that lands that same goal in that pane marks the entry delivered; an
unrelated line typed into the pane does not.

`gtmux tasks --pending` is the pending-decision standing view, what is on your plate:

```
▸ t1kx8p2m9dq3  hq %21                 08-09 14:32  ship v0.48.0 or hold for §4?
▸ t1kx9r4w0aa1  worker-b %8            08-09 09:11  which branch should the migration target?
```

The leading glyph is `▸` (an entry on this list is awaiting a decision). Ordering is
the oldest wait first, then pane, then id. The view reads the ledger only and prints an
absolute stamp, so two reads of an unchanged plate are byte-identical.

Entries go on and off the plate with `gtmux tasks --await <task_id>` and
`gtmux tasks --resolve <task_id> [disposition]` (the disposition records how it left:
`decided` / `withdrawn` / `escalated`; omitted, it just clears). Membership is the
`awaiting-commander` disposition, so any other disposition also takes an item off the
plate, and archiving an entry closes it out of the view.

### `gtmux reap`

`gtmux reap <pane|task_id>` safely reclaims a finished dispatch. It runs a safety gate
first (the worktree must be clean and the branch merged) and only then kills the session,
removes the worktree, and deletes the merged branch; when the gate fails it reports
exactly what blocks it and touches nothing (`--abandon` overrides, `--keep-branch` keeps
the branch). A step that fails is named with git's own reason under `⚠ but these steps
failed`, and the command exits non-zero. `--snooze [--for <dur>]` silences a reap
suggestion for a dispatch you're keeping. When a tracked dispatch looks reclaimable, a
live HQ gets a `» gtmux·reap-suggest … │ gtmux reap <id>` wake. Reclaim is always
suggest → approve → execute, never automatic.

A suggestion needs more than an idle pane. The ledger holds only what HQ dispatched, so a
session you drive yourself never enters it and goes on showing the one old task it was
spawned for; half an hour of quiet used to be enough, and gtmux once proposed reclaiming
two live flagship sessions in the same minute. A suggestion is now made only when the
record shows that every prompt that pane has received since the dispatch was put there by
gtmux itself. Where that cannot be shown, none is made: a withheld suggestion costs a pane
that lingers, an acted-on one costs a session's whole context.

## `gtmux usage`: token watch

```
PLAN   % used, and when the window comes back
  claude 5h                   16% ██░░░░░░░░░░   back in 2h 24m  Jul 13 at 1:29am
  claude week (all models)    74% ████████░░░░   back in 4h 34m  Jul 17 at 10:59pm

CONVERSATIONS  8                                  out    ctx   rate
  each conversation since it started
  ⠿ api:0.0                                      2.1M    85%   7k/m   ⚠ ctx 85%
  ⠿ web:0.0                                      830k    60%  391/m
    … 5 more idle, 50k between them

TOTALS   every agent on this Mac
  today                     2.8M
  this week                16.2M   claude 15.1M · codex 1.1M
  since Jun 12              233M   busiest day 9.4M · 23 days running, best 31
```

The plan leads: it is the one number local counting cannot produce, and it decides
whether you can keep going at all. The conversation list keeps its head and folds its
tail, and the column words are said once in a header that also names the PERIOD — one
conversation's own total can be larger than the whole week's, because the conversation
is older than the week, and the two numbers contradict each other until something says
so.

Three different things used to be called a session here. A tmux session is what
`overview` counts and `restore` brings back; a conversation is one agent's ongoing
chat, which is what this list holds; and Claude's rolling five-hour allowance is named
by its length, `claude 5h`. `--json` still carries the agent's own label.

The `today` and `this week` lines total tokens by local day across every agent; each
message is attributed to the day it happened. `--json` carries the last seven days under
`history`, and the ledger's whole year under `history.activity` (every day with output,
the total since the ledger's first day, the peak, the streak). A second
`Σ all … since … · peak … · streak …` line says the year in one line, and
`gtmux usage --activity` draws it as the calendar heatmap the phone and the Mac reader
show (weeks across, Monday to Sunday down, GitHub's five greens; as many weeks as the
terminal is wide, `COLUMNS` respected):

```
Token activity   last 26 weeks
all 70.8M · peak 5.3M · streak 14d (best 25d)

      Apr     May       Jun       Jul       Aug       Sep
Mo  · · · · · · · · · · · · · · · · · · ░ ░ ░ ▓ ░ · █ ▓
    · · · · · · · · · · · · · · · · · · ░ ░ ░ ▒ ▒ · ▒ ░
…
  Less · ░ ▒ ▓ █ More
```

Per-session token accounting is parsed deterministically from the agent's own log (zero
LLM calls): cumulative output/input, the live context footprint (the last message's input
+ cache tokens, judged against an evidence-inferred window), and a 10-minute spend rate.
Layered thresholds per agent type live in `~/.config/gtmux/usage.json`:

```json
{"claude": {"ctxWarn": 0.8, "sessionOutWarn": 20000000,
            "typeRatePerMinWarn": 30000},
 "horizonMin": 30}
```

The evaluator also projects (`current + rate × horizon`) so you are warned before a
wall (`ctx→80% in ~9m`). Warnings surface as an amber `usage_warn` on the radar row
(`agents --json` / digest), in `gtmux usage`, and as a one-per-layer
`» gtmux·usage·warn …` wake into a live HQ session. `--json` is also served as
`GET /api/usage`. The hook evaluates on every lifecycle event: near-real-time during
tool-driven work; a long silent generation settles at its next event.

Claude records what each message cost, so its totals are a running sum. Codex records
the session's running totals after every turn, so its totals are the last reading, and
it states `model_context_window` outright, so its ctx fraction is measured against the
real window. An agent whose log carries no usage at all still gets a row, with these
fields empty.

> Network-aware launch: gtmux prefixes agent launches (`gtmux hq` / `adopt` / restore /
> the limits command) with a proxy when needed, so you never hand-toggle one across
> networks. `~/.config/gtmux/config.json` → `"agentProxy": "auto"` (default) applies
> `http://127.0.0.1:<agentProxyPort, 7897>` only while that port is listening (your proxy
> tool is running, the home-VPN case) and nothing otherwise (intranet); an explicit URL
> forces it, `"off"` disables.

## `gtmux events`: the session event stream (subscription)

```
22:50:40  working          api:0.0        Claude Code (%7)
22:51:02  waiting·permission  api:0.0     Claude Code (%7)
22:53:19  idle             web:1.0        Codex (%11)
```

The hook appends every session's lifecycle event (start / finish / waiting / background)
to a rotated log (`~/.local/share/gtmux/events.jsonl`, active 20 MB + 1 rotated ≈ 40 MB
ceiling, `eventsCapMB` config; `0` disables). `gtmux events` prints the last hour;
`--since 10m|2h` a window; `--follow` streams live and is rotation-aware. `--since-seq N`
is the one-shot delta read (everything strictly after sequence N, oldest first,
combinable with `--severity`/`--json`): HQ is woken by a signal line naming a sequence
range and pulls exactly that delta, on any agent that can run a CLI command. This is the
terminal-native subscription to the same events the apps get over SSE.

An unfiltered `--since-seq` read run from the HQ home also advances the watermark to
the end of what it returned, which is what stops the `unread` knock (see
[The watermark](#the-watermark-why-nothing-goes-missing)). `--ack N` writes the
watermark back explicitly, for when the stream was reconciled some other way, e.g. a
full `gtmux digest`. Both are HQ-only (cwd-keyed); a worker running `gtmux events` in a
repo changes nothing. The rule is the exact cwd: a read from a subdirectory of the HQ
home (`notes/`, `knowledge/`) does not count and warns on stderr, naming the home to run
from; stdout and the exit code stay the same.

HQ's own delta pull omits the records that never counted (HQ's own pane's lines,
pane-less blinks, and gtmux's `gtmux:audit:*` trail) and says on stderr how many it
withheld. `--all` restores the raw view; both forms consume. Anyone else's read is
unchanged.

`--severity <tier>` filters to that tier and above. The tiers rank urgency, so they are
three different reads: the unfiltered `--since-seq` delta is what you reconcile with;
`--severity notable` is the fleet-change stream (an instruction reaching a session,
`origin:"instruction"`, plus turn-ends and lifecycle); `--severity important` is the
escalation subset (blocked · asking · crashed), to triage first.

`--acts` keeps only the supervision's own acts (dispatches, reaps, knowledge writes,
rotations, self-checks, distillations) and drops the wake plumbing
(`wake-delivered` / `wake-dropped`). It is the same partition the phone's "HQ's work"
section reads over `GET /api/hq/events?acts=1` and the menu bar's "HQ did" row counts;
"what did HQ do today" is `gtmux events --since 24h --acts`. Like every filtered read, it
never counts as consumption.

The stream also carries gtmux's own control records, the periodic maintenance triggers
it raises for HQ, rendered as `[CONTROL <event>]` with their reason:

```
09:57:16  [CONTROL gtmux:self-check]  due (daily) — review feed/ledger/memory health…
04:33:49  [CONTROL gtmux:distill]     due (weekly) — distil the period into the KB…
```

So "did the periodic pass actually run?" is `gtmux events --since 30d | grep distill`.
`gtmux doctor`'s HQ maintenance rows show when each pass last ran and flag one that has
slipped past its cadence (shown only on a machine that has an HQ home).

## `gtmux resource`: local machine resource watch

```
disk 40GB free · mem 38% free (warn) · load 0.64×14 cores · power 74% (battery 2:13)   ⚠ disk 40GB free
per-agent (RSS · CPU):
  %26    252MB · 9.2%
reclaim candidates (orphans no live agent owns):
  pid 3015  100MB · 0.0%  iOS Simulator runtime (12 procs) [simulator]
    ↳ leftover iOS Simulator runtime — `xcrun simctl shutdown all`
```

Disk (`df`), memory (`memory_pressure -Q` free % + the kernel
`kern.memorystatus_vm_pressure_level` normal/warn/critical tier), CPU (loadavg÷cores),
and power/battery (`pmset -g batt`: charge % · on-AC vs draining · time left; absent on
a battery-less host; a low charge counts toward the warn/tier only while draining, never
on AC). Per-agent RSS/CPU by walking each pane's process tree, and reclaim candidates:
heavy processes no live pane owns, named with pid plus how to reclaim (a leftover iOS
Simulator runtime aggregates into one entry; dev servers surface individually). Thresholds live in `~/.config/gtmux/config.json`'s `resource` object
(diskAmberGB 50 / diskRedGB 15 / loadAmber 1.0 / loadRed 1.5 / orphanRssMB 300 /
batteryAmberPct 20 / batteryRedPct 10). A resource block rides `GET /api/usage`; the
serve tick emits a `resource·warn` nudge to HQ (one per crossing); `gtmux hq`/`new` warn
at a red line before adding load.

A candidate first has to survive one question: if this ends, what ends with it? Whatever
the running work stands on — the tmux server every session lives inside, gtmux's own
resident processes, the agents themselves — is never offered, whatever its size. The
warning once read `disk getting low · 29GB free — maybe reclaimable: tmux`, and acting on
that frees nothing and stops every piece of work on the machine at once. Such a process
reaches the list for a structural reason, not by accident: the list looks for something
large that no task claims, and the floor is long-lived, sizeable and claimed by nothing
precisely because it is the floor.

A candidate rides the warning only where ending it would help. It is a process, so its
size is memory: the suggestion accompanies a memory or load warning, names what it holds
and in which resource, and stays away from a disk or battery warning entirely. Killing a
902MB process returns no disk, and the line that suggested it during a full disk was acted
on twice before anyone noticed (#1109).

The warning is damped three ways so a value sitting on a threshold can't re-alert (the
readout itself stays raw):

| key | default | what it does |
|---|---|---|
| `diskHysteresisGB` | 2 | GB of headroom above the entry line before a disk tier clears (red at <15 GB clears at ≥17) |
| `loadHysteresis` | 0.15 | load÷cores below the entry line before a load tier clears (amber at ≥1.0 clears below 0.85) |
| `batteryHysteresisPct` | 3 | % of charge above the entry line before a battery tier clears (amber at <20% clears at ≥23%) |
| `confirmSamples` | 3 | consecutive agreeing samples before a tier change is believed |
| `minRestateMinutes` | 30 | quiet period before the same tier warns again; an escalation to a worse tier is exempt and always warns |

## `gtmux limits`: real subscription-window remaining

```
% used, and when the window comes back
  claude 5h                   16% ██░░░░░░░░░░   back in 2h 24m  Jul 13 at 1:29am
  claude week (all models)    74% ████████░░░░   back in 4h 34m  Jul 17 at 10:59pm
  claude week (fable)        100% ████████████   back in 4h 34m  Jul 17 at 10:59pm
  codex week                   0% ░░░░░░░░░░░░   back in 2d 15h  Jul 20 at 10:02am

  claude week (fable) is spent until Jul 17 at 10:59pm. Claude Code keeps answering;
  that window is at 74%.
read just now
```

A bar per window, the word "used" said once, and a closing line only when one window is
at its cap — saying when it comes back and what still works meanwhile, rather than
repeating the number on the row above it.

How much of your plan is left, as real server data from what the agent itself reports.
Claude and Codex report it in different places:

- Claude has nothing about windows on disk (its transcript holds one conversation's cost, its stats
  cache all-time model totals), so gtmux runs the agent's own command headlessly:
  `claude -p "/usage"`.
- Codex writes the server's rate-limit response into its session rollout, beside the
  token counts, so gtmux just reads it. No process, no command.

Two rules apply to the log route. A window is named by its duration, never by its
position in the source (Codex's `primary` field is observed carrying the weekly window
as well as the 5-hour one). And a window whose reset has passed is dropped, since a log
is only as fresh as its last turn.

When Codex has no readable window but you used it in the past week, it gets a line of
its own:

```
● claude 5h                   9% used   resets Sep 7 at 9:09pm
● claude week (all models)   50% used   resets Sep 11 at 10:59pm
○ codex  the window it last reported has ended — codex writes its plan into its own log, so one turn brings the figure back
```

`gtmux usage`'s footer flags the same gap as `codex unknown`. An agent you have not used
in a week says nothing at all.

`gtmux limits` lists every window. Every other place (the `gtmux usage` footer, the
phone's header row) shows one per plan, the tightest. The warning rule is different: it
ignores 5-hour windows, which reset on their own.

Every window says whose plan it is, the first agent's included: `claude 5h`,
`codex week`, never a bare one. The `spawn` preflight prints the warning, so it
names the plan the work will bill against. Because the Claude route spawns a process,
results are cached with a 15-minute TTL, shortened to 5 minutes once any window is near
its cap; `--refresh` forces one. Configure in `~/.config/gtmux/usage.json`:

```json
{"limitsCommand": "claude -p /usage", "limitsTTLMin": 15,
 "limitsTTLNearMin": 5, "limitsNearPct": 70, "limitsWarnPct": 85,
 "limitsTimeoutSec": 60}
```

Set `limitsCommand` with an env prefix if your network needs it
(`"HTTPS_PROXY=… claude -p /usage"`), or `""` to disable. A run that outlives
`limitsTimeoutSec` is killed. A run that fails is never cached as fresh, so the plan
figures you already have are kept instead of blanked, and the command backs off (1, 2,
5 minutes, then the TTL) instead of being retried by every caller. A weekly window
at or over `limitsWarnPct` marks amber and wakes a live HQ once
(`» gtmux·limits·warn …`). The `limits` block also rides `gtmux usage` and
`GET /api/usage`.

## `gtmux logs`: what gtmux saw and what it did

Every gtmux process writes to one local store, in the spirit of the macOS system log:
serve, the tunnel client, the hook, every command and the menu bar. It holds two kinds of
entry. Diagnostics say what gtmux saw. Actions say what it did to this Mac, who started it,
what it acted on, and how it ended (`ok`, `refused` with a reason, or `failed` with the
error). The actor is `user` for a command you typed, `hq` for one HQ ran, `agent:%7` for
one an agent ran from pane %7, `menubar`, a phone or browser by device (`phone:3f9c20e1`),
a share link (`guest:…`), or `system` for what serve and the hook do on their own.

<!-- gtmux:rendered log-lines -->
```
09:36:05 serve   serve.start  serve started · backend=direct port=8765
09:41:12 serve   act.send  phone:3f9c20e1 → %7 ok · bytes=42 via=tunnel
09:44:02 serve   warn  act.pair  anonymous refused · a pairing code was not accepted · reason=expired via=tunnel
```

```sh
gtmux logs                                   # the last hour
gtmux logs --since 1d --acts --actor phone   # everything a phone did today
gtmux logs --event 'act.pair' --since 2h     # each pairing attempt, and why one was refused
gtmux logs --level warn --since 3d           # warnings and errors, refused actions included
gtmux logs --follow                          # new entries as they arrive
gtmux logs --json --since 10m                # raw entries, for scripts and agents
gtmux logs --since 1d --stats                # how much is kept, and how much of today went wrong
```

`--stats` answers about the store instead of printing it: its size and oldest day, the
retention in force, and how many entries in the window were warnings or errors. With
`--json` it is one object, which is what the menu bar reads for its Diagnostics section.

A refused pairing names one of three reasons: `expired` (the code's 5 minutes ran out),
`used` (a code works once), or `unknown` (this serve never issued it, which is what a code
minted before a restart looks like).

Every action has a stable event name, which is what `--event` matches. This is all of
them, each with the commands that record it (`serve` is what serve does for a phone, a
browser, a share link or the CLI; `hook` is the agent hook; `app` is the menu bar app):

<!-- gtmux:rendered act-catalog -->
```
act.adopt            adopt
act.app.launch       app
act.attach           attach, serve
act.awake.off        awake
act.awake.on         awake
act.capture          capture
act.cleanup          doctor, serve
act.config.set       config, quiet
act.doctor.bundle    doctor
act.doctor.fix       doctor
act.focus            focus, serve
act.hq.brief         hq
act.hq.export        hq
act.hq.import        hq
act.hq.rotate        hq
act.hq.start         hq
act.install.app      install
act.install.hooks    install
act.knowledge        knowledge, serve
act.knowledge.sync   knowledge, doctor
act.mint             pair, serve
act.narrow           serve
act.new              new
act.notify           hook
act.notify.post      app
act.pair             serve
act.push.forget      devices, serve
act.push.register    serve
act.reap             reap
act.reap.snooze      reap
act.restore          restore
act.resume           restore
act.revoke           pair, devices, share, serve
act.send             send, serve
act.share.config     share, serve
act.share.create     share, serve
act.share.set        share, serve
act.spawn            spawn
act.tunnel.off       tunnel
act.tunnel.on        tunnel
act.tunnel.move      tunnel
act.tunnel.redeem    tunnel
act.uninstall.app    uninstall
act.uninstall.hooks  uninstall
act.unwatch          panes
act.update           update
act.upload           serve
act.wake.delivered   serve, hook
act.wake.dropped     serve, hook
act.watch            panes
```

restore writes its reasoning here too, always: which save it picked and which conversation
each pane was matched to (`gtmux logs --component restore --since 1d`).

Entries are English and never hold message text: a send records its length and a short
hash. Tokens, pairing codes and `Authorization` values are replaced where the entry is
written. The store is `~/.local/share/gtmux/logs/`, one file per day, readable by you
only. It keeps 30 days or 100 MB, whichever comes first (`logs.retainDays` and
`logs.maxMB` in `~/.config/gtmux/config.json`). Whichever process writes the first entry
of a day also removes what has expired, so the store stays bounded without serve. A day
that passes 20 MB starts a second file, and one `log.runaway` entry names what filled it.
`GTMUX_DEBUG=serve,tunnel` (or `all`) adds debug entries for one run; `"debug": "hook"` in
`config.json` does it for every process, including the ones launchd starts. What a daemon
prints before it can log, a crash for instance, goes to `logs/<component>.stderr`, which
serve's sweep caps.

The menu bar has the same three things without a terminal. Preferences › Diagnostics says
how much the store holds and how many of today's entries went wrong; **Open** shows the
last three days as a list, newest first, switchable to problems only; **Pack…** runs the
bundle below and says where the file landed; and **Record extra detail** is `gtmux config
debug` (each process picks it up when it next starts, so turn it off when you are done).

`gtmux doctor` has a Logs section: the store's size and oldest day, a runaway writer in the
last week, errors in the last day, whether any file gtmux keeps is readable by another
account on the Mac, and the other stores against their bounds. `gtmux doctor --fix` runs
the cleanup and narrows file modes. Nothing here is uploaded anywhere.

To report a problem, `gtmux doctor --bundle` packs one file: the log store, the status
files, the last 256 KB of each launchd capture, the doctor report as text and the
versions of gtmux, the app, macOS and tmux. It lists what it packed. Every token and
pairing code gtmux keeps is replaced again on the way in, including in the launchd
output that never went through the store. The event journal is left out because it holds
the heads of your prompts; `--with-events` adds it. The file is readable by you only,
and where it goes is up to you.

```sh
gtmux doctor --bundle                  # gtmux-diagnostics-20260920-0930.tgz here
gtmux doctor --bundle ~/Desktop/r.tgz  # a path of your own; an existing file is never replaced
```

The phone keeps its own record of the same kind: its failed requests to the Mac, each
pairing attempt and why it failed, push registration and the live stream dropping and
coming back, the last 500 entries or 200 KB. It stays on the phone. Settings → Diagnostic
record opens it: each entry as a sentence ("Could not reach the Mac · GET /api/agents did
not answer after 6s, then 4 more times in a minute"), grouped by day, with a problems-only
filter. Copy or Share hands the record over untranslated, as JSON lines in the shape
`gtmux logs --json` prints, so both sides of the same minutes read together.

## `gtmux awake`: keep working with the lid closed

```
gtmux awake on       # asks for your admin password once, then verifies it took effect
gtmux awake          # awake = on (clamshell) · up 2h13m · power battery 74%
gtmux awake off      # no password — immediate
```

Closing a MacBook's lid sleeps the system, which drops the tunnel and freezes every agent
mid-turn. `gtmux awake on` keeps the Mac awake with the lid shut (`gtmux server-mode`,
the old name, still works). Turning it on costs one password; turning it off costs
nothing and works even when gtmux is dead.

A small root-owned guard is installed in the same authorization. Its only power is to
give sleep back: it restores sleep and deletes itself when any of these happens:

| trigger | what it means |
|---|---|
| you turn it off | an unprivileged marker; the guard wakes on it within about a second |
| charge reaches 20% | you are warned at 30%, on the Mac and on your phone |
| gtmux stops running | crash, force-quit, `brew uninstall`: nothing needs to survive |
| a reboot with nobody logging in | after a startup grace, so a normal restart does not kill your session |

**It never expires.** It runs until you turn it off, and the menu-bar icon carries a
slowly pulsing red dot the whole time, the same visual language as a screen recording.

Battery is a supported case: carrying a closed laptop between rooms keeps working. What
ends it is remaining charge, not losing the adapter.

gtmux reverts only what gtmux set. A `disablesleep` it did not stamp is reported with
the manual undo command and never changed for you. `gtmux doctor` surfaces the same
finding, and stays silent on machines that have never touched the setting.

Where the state is read from:

| source | use it? |
|---|---|
| `pmset -g` / `-g custom` / `-g live` | ❌ never reports `disablesleep`, in either state |
| the power-management plist | ⚠️ lags a write; answers "would it survive a reboot" |
| `ioreg -r -c IOPMrootDomain` → `SleepDisabled` | ✅ the live, unprivileged truth |

`gtmux awake --json` reports both readings plus `owned_by_gtmux`, `guard`, and a
`platform` verdict. On a macOS the project has not verified, `on` says so; where the
mechanism is absent it refuses before asking for a password.

Two boundaries:

- `gtmux serve` is a per-user LaunchAgent, so after a reboot it starts only once someone
  logs in. On a FileVault Mac with nobody there the heartbeat never resumes and sleep is
  restored, so server mode does not survive an unattended reboot. gtmux will not "fix"
  that by touching FileVault or auto-login.
- The underlying setting is undocumented by Apple. It is verified on macOS 26 and
  detected at runtime; if a future macOS drops it, `on` refuses with a reason.

Your phone can see this state (a ring on the connection dot, and a row in Servers /
Manage Mac) but never change it: every path to changing it ends at a password typed at
the Mac.

## `gtmux restore`

Quitting your terminal leaves the tmux server and all sessions alive; only the tabs are
gone. After reopening, run once in any tab:

```sh
gtmux restore            # one terminal tab per tmux session, all attached
gtmux restore --pick     # choose which sessions: "1 3" / "1,3", Enter = all, q = cancel
gtmux restore --one      # attach the next unattached session in this tab
gtmux restore <name>     # attach a specific session here
gtmux restore --dry-run  # print what would happen, change nothing
gtmux restore --plan     # preview: which sessions + agent conversations would come back (read-only)
gtmux restore --plan --json   # the same plan as JSON (the menu bar's source for its expandable restore row)
```

One restore at a time: a run holds a lock (pid + start time) and a second run says so
and does nothing. A lock whose process is gone, or older than 10 minutes, is taken over.
`--plan` and `--dry-run` are exempt from the lock.

A real `gtmux restore` prints its plan up front: the sessions it is about to bring back
and the agent conversation (goal) under each pane. `--plan` is that preview on its own:
it reads the last resurrect save plus the resume records and starts no tmux (safe to run
or poll anytime). An agent line marked `×` is a conversation whose transcript is gone
from disk and will not resume.

Only a pane that was running an agent when the layout was saved gets one back; restore
reads that from the save's own record of each pane's command. A pane that was a plain
shell at save time comes back a plain shell, even if you ran an agent in it last week.
The conversation a pane gets is its resume record; if that is missing, restore reads the
id out of the `--resume` the save recorded it running.

Restore always prints the moment it is putting back, for example "Restoring the layout
saved at 09:57 (37m ago)". The autosave that writes that file hangs off tmux's status
bar, so it only runs while a terminal is attached and redrawing: close the lid and it
saves nothing. `gtmux serve` backstops it by watching the file: if nothing has written
the save for about 10 minutes (about 20 when an autosave trigger is present) serve runs
the save itself. `gtmux doctor`'s `resurrect autosave` row flags an armed trigger that
has not saved for hours.

After a restore, gtmux compares every saved window's pane count and arrangement against
the live one and names any that differ, on the terminal and in the log store
(`gtmux logs --component restore`), since tmux-resurrect discards its own layout errors.

A tmux pane id is a per-server sequence number: restart the server and `%25` is handed
to a different pane. gtmux keys a lot of state by that number, so restore (and, every
few minutes, `gtmux serve`) drops pane-keyed records whose panes are gone. Conversation
records (`resume/`, `usage/`) are keyed by locator and conversation id and are never
touched.

To diagnose a restore without a reboot, point `XDG_DATA_HOME` at a copy of any save and
preview it read-only:

```sh
mkdir -p /tmp/probe/tmux/resurrect && cd /tmp/probe/tmux/resurrect
cp ~/.local/share/tmux/resurrect/tmux_resurrect_<stamp>.txt . && ln -sf tmux_resurrect_<stamp>.txt last
XDG_DATA_HOME=/tmp/probe gtmux restore --plan     # what restore would bring back from THAT save
```

The first run pops an Automation permission dialog ("wants to control Ghostty", or
iTerm2/Warp, whichever hosts your tabs); click Allow. After a reboot the tmux server is
gone too; `gtmux restore` starts tmux and explicitly drives
[tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) to restore the last
autosave (it waits for the restore to finish, large layouts take 30 s or more, and if a
saved layout exists but can't be restored it refuses to overwrite it). Running programs
are not restarted; relaunch e.g. with `claude --resume`.

Each pane's previous output (scrollback) comes back too, as a snapshot, when resurrect
is set to capture it. Recommended in `tmux.conf`:

```tmux
set -g @resurrect-capture-pane-contents 'on'   # snapshot each pane's scrollback
set -g history-limit 50000                     # how much scrollback to keep/restore
```

> The shell's ↑ command history is separate: it lives in your shell's histfile, not in
> resurrect. By default it is written only on shell exit, so a reboot loses recent
> commands. To persist it immediately (bash):
> `shopt -s histappend; PROMPT_COMMAND='history -a'` in `~/.bashrc`
> (zsh: `setopt INC_APPEND_HISTORY`).

The history behind these rules (phantom agents after a reboot, silent layout failures)
is in [TROUBLESHOOTING](TROUBLESHOOTING.md#restore-phantom-agents-and-silent-layout-failures).

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

A sessions/windows/panes summary from any shell. `--popup` is size-fitted for a tmux
`display-popup`, so you can bind it to a key and float it over a full-screen program
without interrupting it.

## `gtmux new`

```
gtmux new                    # a session named for the current directory
gtmux new api                # …named api
```

Creates a tmux session and opens a terminal tab attached to it, through the same
terminal driver `focus` and `restore` use, so the tab lands where you can see it instead
of in a detached session you then have to go find.

## `gtmux adopt`

```
gtmux adopt 4f0c1a2b                 # bring that conversation into a new tmux session
gtmux adopt 4f0c1a2b 91de77c4        # several at once
```

An agent started outside tmux is sensed read-only (the Elsewhere section: its hook fires
with no `$TMUX_PANE`, so gtmux knows it exists but has no pane to show, jump to, or type
into). `adopt` resumes the conversation by session id inside a fresh tmux session, and
from then on the row is a full one. Take the id from `gtmux agents --json`
(`session_id`) or the radar row. Only agents whose CLI can resume by id are adoptable;
the rest are listed and left alone.

## `gtmux focus`

```sh
gtmux focus web          # bring the terminal tab showing session "web" to front
gtmux focus %11          # jump to that exact window+pane, then focus its tab
```

Each tab title is `session — window`, so `focus` finds the matching tab and brings it to
front (via the terminal's AppleScript). A pane id (`%N`) also selects that window+pane
inside the session, so you land exactly where the agent is, which is how a notification
click drops you on the agent that just finished.

A session with no window open (a `--headless` spawn, or one you detached) has no tab to
bring forward, so `focus` opens one and attaches it. The test is the session's client
count, not how the session was started: a headless session someone attached later is an
ordinary jump. Surfaces mark such a row (`no window` / `无窗口`) so you know a tab will
open before you click.

> Needs `set-titles on` with `set-titles-string '#S — #W'` so tab titles stay in the
> format `focus` matches. If another tool also writes the tab title, disable that so
> titles stay authoritative.

Host terminals: Ghostty and iTerm2 are fully driven (exact tab via AppleScript). Warp is
best-effort: it has no AppleScript dictionary, so `focus` jumps to the exact tab only
when that tab's Warp session uuid was recorded into the tmux session env (gtmux's own
restore/new attach does this; or add `WARP_TERMINAL_SESSION_UUID` to tmux's
`update-environment` to cover hand-opened tabs), and otherwise just activates the Warp
app; `restore`/`new` open Warp tabs via launch configurations. Other terminals fall back
to the Ghostty driver. The host is auto-detected; `GTMUX_TERMINAL=ghostty|iterm2|warp`
overrides the detection.

## `gtmux attach`: work in a remote session from another machine's terminal

Where `focus` jumps to a local tab, `attach` opens a remote pane in your current terminal
(Ghostty / iTerm2 / Terminal) as a raw, interactive passthrough: the local terminal
becomes the remote tmux session, over the same `gtmux serve` surface (a WebSocket,
`GET /api/attach`), honoring the owner/guest token scope.

```sh
# owner — full access with the serve token:
gtmux attach http://<mac>:8765 --token <serve-token> %12

# guest — a scope-restricted share link (from `gtmux share new`, or the menu bar's
# Sharing → New link); attach exactly what the host allowed:
gtmux attach 'https://<mac>.example#code=4F7K-Q9X2' %12
gtmux attach 'https://<mac>.example' --code 4F7K-Q9X2   # same link, read out to you

gtmux attach <target>            # omit the pane: auto-attach the only one, else pick
gtmux attach <target> --read-only  # watch only, never send input
gtmux attach <target> --predict    # experimental: hide round-trip lag while typing
```

`--predict` (experimental, off by default) is predictive local echo, the mosh idea
adapted to the WebSocket bridge. Over a slow link every keystroke otherwise waits a full
round-trip to echo (about 340 ms on a cross-continent tunnel). With `--predict`, your own
printable typing and backspaces appear immediately, underlined to mark them unconfirmed,
and are erased the instant authoritative output arrives; the server screen always wins.
The real keystroke is forwarded to the pane unchanged. Nothing is drawn on a fast/LAN
link, nothing inside a full-screen TUI (the server tells the client when the pane is on
the alternate screen), and any state-changing key (Enter, ESC, arrows, Ctrl-C, Tab)
ends the prediction epoch. The client learns the cursor from the server; see
`docs/design/mosh-predictive-echo-research.md`.

- `<target>` is a host (plus `--token`, which makes you the owner, full access) or a
  `…#code=<code>` share link (a guest, restricted to the host's view/input allowlists: a
  view-only pane is read-only, and a non-viewable pane is refused). `--code` takes the
  link's code on its own, for when someone read it out to you.
- `%N` (optional) is the tmux pane id to attach; it selects the session that pane is in.
  Omit it to auto-attach when there is a single session, or (on a TTY) pick from a
  numbered menu (session · agent · status · task per row; Enter takes the first row, `q`
  cancels). Piped or scripted (stdin not a TTY), it prints the list and exits non-zero.
- Detach with tmux's own `<prefix> d`, or `Ctrl-]` (the local escape hatch).
- Scope is enforced server-side; `--read-only` is a convenience, not the security
  boundary. See `docs/design/remote-attach-research.md` for the design and trade-offs.

> Needs the host reachable: on the LAN directly, or from anywhere via `gtmux tunnel`
> (the WebSocket rides the same tunnel as the radar). The guest side is set up entirely
> in the menu bar (per-pane 👁 See / ⌨️ Type + New link) or with `gtmux share`.

## `gtmux pair`: enroll your own devices (full control)

```
gtmux pair                  # mint ONE one-time code, printed three ways:
                            #   phone QR · browser https://…/#c=<code> · a one-line
                            #   `gtmux attach '<url>/#c=<code>'` for another terminal
gtmux pair list             # your paired devices (guests live under `gtmux share`)
gtmux pair revoke <id>      # cut one device off, effective immediately
```

Pair is the owner track of the pair/share model: the enrolled device is you, with full
view and input on every session. All three media redeem the same short-lived code
(5 min, single use) into the same revocable roster. The terminal medium persists its
token in `~/.config/gtmux/remotes.json` (0600), so afterwards a bare
`gtmux attach <host>` just works; `pair revoke` invalidates it instantly. The link
carries the tunnel URL when `gtmux tunnel` is up, else a LAN address. `gtmux devices`
remains as an alias of the roster.

### `gtmux devices --push` / `--forget-push`: inspect and clean up push tokens

```
gtmux devices --push                       # roster annotated with each device's push
                                           #   token (✓ env·kinds) + any UNLINKED tokens
gtmux devices --forget-push <id|orphans|all>  # drop push tokens (host-only)
```

Push tokens are bound to the enrolled device that registered them, so
`gtmux devices revoke <id>` already stops that device's notifications. `--push` shows
the binding; `--forget-push` clears tokens by selector: a device `id`, `orphans` (only
unlinked legacy tokens, from before device-binding), or `all`. Use `orphans` when a
removed phone keeps getting notifications from a stale token an old app never
unregistered. Host-only (the local master token); a remote device or guest is refused.

## `gtmux share`: scoped, revocable access for a collaborator

```
gtmux share new --label Alice --view %1,%2 --type %1 --expires 24h
gtmux share set a1b2c3d4 --type %2            # edit ONE link (omitted flags untouched)
gtmux share link a1b2c3d4 [--json]            # re-show an existing link, both ways (+ QR)
gtmux share on|off                            # consent master switch for ALL guest typing
gtmux share status [--json]                   # per-link scope summaries
gtmux share revoke a1b2c3d4
```

Share is the collaborator track of the pair/share model: a guest link with least
privilege. Each link carries its own scope: which panes the guest may see (`--view`) and
type into (`--type` ⊆ view), plus an optional expiry (`--expires 45m|24h|7d`, default
never; expired links fail auth like revoked ones). Typing additionally needs the host
consent (`share on`, default off). Everything is enforced server-side; the web page and
app only mirror it.

A link minted without `--view/--type` copies the current global lists (the template).
The legacy global forms (`share add/remove`, `share view add/remove/clear`) still work
but fan out to every existing link; per-link tailoring uses `share set`. `status --json`
carries each guest's `view_panes`/`panes`/`expires_at` and never a bare token, plus who
has used the link: `last_seen`, `platform` (`Chrome 141 · macOS`), `last_ip`, all absent
until someone has. `gtmux share link <id>` (or the menu-bar row's copy button) re-hands an
existing link at any time, full-scope callers only.

### What a share link is, and what happens on each end

A share link is one collaborator's access to this Mac: the panes they may watch, the
shorter list they may type into, and an optional expiry. Each link carries its own access,
so you can hand out three and revoke one.

The link ends in a short code, and that code is the link:

```
https://tunnel.example.dev/p35047#code=4F7K-Q9X2
```

Send it and they click it. Where nothing can be pasted, at a TV browser or a locked-down
machine or over the phone, read them the two halves instead:

```
https://tunnel.example.dev/p35047
4F7K-Q9X2
```

Both reach the same place. The code lasts exactly as long as the link does, so there is
one thing to keep track of and one thing to revoke. Guessing it is bounded at the door:
too many wrong codes and the door stops answering for a minute.

In a browser, they open the link, or open the address and type the code into the box on
the entry page. The browser keeps the access from then on, so coming back tomorrow just
works. They see the panes on the view list, and can type into the shorter list while your
consent switch is on (`gtmux share on`). They cannot reach anything else on the Mac.

A terminal does the same job through `gtmux attach <link>`, or `gtmux attach <host> --code
4F7K-Q9X2` when the link was read out. It keeps the access for that host, so later it is
just `gtmux attach <host>`. With one pane in their scope it attaches to that one; with
several it asks which. A pane they may watch but not type into attaches read-only and says
so on the line above the session.

`gtmux share revoke <id>` cuts both ends at once: the browser falls back to its entry page
on its next request, and the terminal's saved token stops working. An expiry does the same
on its own schedule. Your other links keep working.

A browser can lose what it kept, through cleared data, a private window, another browser,
or Safari's rule that clears storage for a site nobody has visited in a week. They reopen
the link and they are back in, with nothing needed from you. A terminal keeps its token in
`~/.config/gtmux/remotes.json`, which stays until you revoke the link or they delete the
file.

Links handed out before codes existed still work. They carry the long token in place of a
code, and open the same access.

## `gtmux whatsnew`: what changed for you

```sh
gtmux whatsnew                 # everything newer than the version you're running
gtmux whatsnew --since v0.36.0 # from a specific version
gtmux whatsnew --all           # every release we have notes for
```

Per release, the lines written for users. `gtmux update` prints the first few of these
after installing; this is the full list. The source is a `user:` block in the release's
tag message, which goreleaser copies into the release body. An optional `user-zh:` twin
carries the same notes in Chinese:

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
(`GTMUX_LANG`): zh prefers `user-zh:`, en prefers `user:`, and either falls back to the
other when a tag carries only one. The blocks may appear in either order; each ends at a
blank line, a heading, or the other block's marker. A release with no `user:` block
contributes nothing.

## `gtmux config`: the few settings that are not per-run flags

```
gtmux config agent-proxy [<url>|off]   # proxy applied when gtmux LAUNCHES an agent
gtmux config tab-alert  [on|off]       # mark the terminal TAB of a session that needs you
gtmux config lang       [en|zh|auto]   # machine-level output language (auto = system locale)
```

Each prints its current value when called with no argument.

### `lang`: one language for every gtmux process

A launchd-started `gtmux serve` and a hook subprocess have no `GTMUX_LANG` and none of
your shell's locale, so without a machine-level choice the wake suffixes and desktop
notifications they emit could disagree with the language your own terminal shows.
`gtmux config lang zh` makes the choice once for all of them; `auto` follows the system
locale. Per-process `GTMUX_LANG` and per-invocation `--lang` still override.

### `tab-alert`: find the right tab without opening nine of them

`tab-alert on` puts a `●` in front of the tab title of any session that has an agent
waiting.

- Only `waiting` marks; `working` and `idle` never do.
- tmux renders the title and the terminal just displays it, so this works on Ghostty,
  iTerm2, Warp, Apple Terminal alike. It is a glyph, since a coloured tab is a
  per-terminal capability.
- Your title format is kept. Enabling reads your `set-titles-string`, stores it, and
  prepends only its own field; `off` restores exactly what it found. If you have edited
  the format since, `off` refuses to overwrite your edit; delete the leading
  `#{@gtmux_alert}` yourself in that case.
- Driven by the agents' own hook events: a waiting marker lands the instant the agent
  reports it, and the serve tick reconciles as a backstop. HQ is not in this loop.
- Also switchable in the menu-bar app's Preferences → Notifications.

### `debug`: record more while chasing something

```sh
gtmux config debug            # what is being recorded now
gtmux config debug on         # every part of gtmux writes debug entries
gtmux config debug serve,hook # just these
gtmux config debug off        # back to the ordinary entries
```

It lives in `config.json` rather than in a shell variable because the processes worth
turning up are the ones no shell reaches: the launchd serve, the tunnel client, the hook.
Each picks it up when it next starts. `gtmux logs --stats` says whether it is on, and the
menu bar's **Record extra detail** is the same setting.

### `hqWake`: tuning HQ's wake channel

Hand-edited keys under `"hqWake"` in `~/.config/gtmux/config.json` (all optional; an
absent or invalid key keeps its default). What each one governs is explained where the
behavior lives: [the wake channel](#the-wake-channel-how-hq-learns-things),
[the watermark](#the-watermark-why-nothing-goes-missing), and
[self-rotation](#self-rotation-when-hqs-own-session-is-the-problem):

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

gtmux is just a CLI; bind whatever keys you like in `tmux.conf`. Suggested:

```tmux
set -g set-titles on
set -g set-titles-string '#S — #W'
bind g run-shell -b "gtmux overview --popup"
bind a display-popup -E -w 80% -h 60% "gtmux agents --watch --popup"
bind J run-shell "gtmux focus --last"
```

### `gtmux status`: the fleet in your tmux status bar

```tmux
set -g status-right '#(gtmux status)'
```

One short, colored run of counts (`●2 ✓14`) for the tmux status line, so the bar you
already have says who needs you without you asking. `--plain` drops the tmux color
escapes for any other status bar (or a shell prompt) that wants the same numbers.

### Put the pane ids in your tab titles (optional)

gtmux names a pane by its tmux id (`%23`) on every surface, and `gtmux focus %23` takes
that id directly. Two lines make the id visible on the other side too, so a row in the
app and a tab in your terminal name the same thing:

```tmux
set -g automatic-rename-format '#{b:pane_current_path} #{P:#{pane_id} }'
set-hook -g pane-exited 'set-window-option automatic-rename off ; set-window-option automatic-rename on'
```

The window name then lists every pane in that window (`gtmux %23 %24`), and since
`set-titles-string` is `#S — #W` the tab inherits it. The title format is unchanged, so
`focus`'s tab matching is untouched.

- `#{P:…}` iterates the window's panes, so the name does not follow focus (why is in
  [TROUBLESHOOTING](TROUBLESHOOTING.md#pane-ids-in-tab-titles)).
- The hook is required: adding a pane re-evaluates the name immediately, closing one does
  not.
- Inside a split window, if you want each pane to wear its id on screen as well:

  ```tmux
  set -g pane-border-status top
  set -g pane-border-format ' #{pane_id} #{pane_current_command} '
  ```

  `doctor` does not suggest this one: `pane-border-status` is `off` by default, and
  turning it on costs a permanent screen row per pane in every split.
- `gtmux doctor` reports this row and `--fix` offers it. If you already have your own
  `automatic-rename-format`, `--fix` appends the ids to it. gtmux does not rename your
  windows: `rename-window` would turn `automatic-rename` off for that window and
  overwrite your format for good.

### Make a printed link clickable (optional)

A program prints a hyperlink as an OSC 8 escape, and tmux forwards it only to a terminal
that claims the capability. No terminal claims it by default, so the link renders as
plain text.

```tmux
set -as terminal-features ',*:hyperlinks'
```

- Needs tmux 3.4+ (`terminal-features` exists from 3.2, the `hyperlinks` feature from
  3.4); on an older tmux the line is a startup error, so `doctor` does not offer it
  below that.
- Only output printed after it takes effect carries a link.
- Ghostty and iTerm2 both handle OSC 8. If a link still does not respond, the next
  suspect is the terminal's own click modifier (some need ⌘ or ⌥ + click).

### Give a plain pane a title worth reading (optional)

An agent writes its own pane title (an agent row reads `整理这周的发布清单和回归结果`);
a shell writes nothing, so gtmux falls back to the command name and every plain row reads
`bash`. Two shell hooks fix it: the title becomes the command while it runs, and the
directory at the prompt.

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

- A tmux pane runs a login shell, and a login bash reads `.bash_profile` /
  `.bash_login` / `.profile` (the first that exists) and never `.bashrc`, so the bash
  version goes in one of those. `doctor --fix` picks the file your shell actually reads,
  and only ever appends to one that already exists (creating `.bash_profile` where you
  keep `.profile` would shadow it).
- Use `add-zsh-hook`; defining a bare `preexec()` replaces whatever you (or oh-my-zsh)
  already had.
- Both are gated on `$TMUX`: outside tmux this would fight your terminal's own tab
  title, which gtmux matches on to jump.
- `gtmux doctor` counts how many plain panes have a title that says anything, whatever
  hook produced it. New shells only; panes already open keep their titles.

## Notification hook

`⏸ waiting`, `✓ latest`, and click-to-jump notifications rely on a hook writing state
files under `~/.local/share/gtmux/`. gtmux ships that hook built in:

```sh
gtmux install                       # asks: hooks | app | all
gtmux install hooks                 # Claude, one-time setup (macOS)
gtmux install hooks --agent codex   # or cursor|gemini|copilot|kiro|opencode
gtmux uninstall [hooks|app|all]     # reverse it (asks when no target)
```

`gtmux install hooks` registers `gtmux hook` in `~/.claude/settings.json` on the `Stop`,
`Notification`, and `UserPromptSubmit` events (idempotent; preserves other hooks and
backs the file up). `gtmux hook` is the producer (Claude Code runs it, you don't) and
writes state purely by event timing, telling a permission request from an idle nudge
without reading message text.

Other agents: `--agent codex|cursor|gemini|copilot|kiro|opencode|kimi` wires that
agent's own hooks file instead. Codex uses its additive hooks system (`~/.codex/hooks.json`
+ `features.hooks`), so it coexists with any existing `notify` (e.g. computer-use)
instead of replacing it. opencode has no command-hook file, so gtmux installs a small JS
plugin (`~/.config/opencode/plugin/gtmux.js`) that forwards its events. Kimi Code keeps
its hooks as `[[hooks]]` entries inside your own `~/.kimi-code/config.toml`, so gtmux
appends one marked block at the end of that file and leaves everything else byte for
byte; uninstall removes exactly that block. `gtmux doctor --fix` offers to wire whatever
agents it detects.

Notifications are delivered by the menu-bar app; no `terminal-notifier` is needed. The
hook queues a request under `~/.local/share/gtmux/notify/` and `Gtmux.app` posts a
native banner (shown as Gtmux, with the agent icon and a Jump action; finished is calm
and silent, needs your input sounds). Clicking it lands you on the exact pane. Grant
"Allow Notifications" on first run and keep the app running to receive them.

### Backing up HQ's records

HQ's records (the whole HQ home) are the one thing gtmux holds that is not reproducible:
the situation board, the knowledge base, and a `LOCAL.md` that is seeded once and never
overwritten (losing it does not self-heal).

```sh
gtmux hq --export ~/gtmux-hq.tar.gz   # the whole records folder as one LOCKED file (asks for a passphrase → .tar.gz.age)
gtmux hq --import ~/gtmux-hq.tar.gz.age   # put one back (asks for the passphrase)
gtmux hq --export ~/gtmux-hq.tar.gz --plain   # the unlocked form
```

`gtmux hq --records [--json]` (`--memory` is the old spelling) says how big the records
are and whether anything at all carries them off this disk. The menu-bar reader shows
the same line, from the same command, so the two surfaces cannot drift about the same
number.

The export is an ordinary tar.gz locked with a passphrase in the
[age](https://age-encryption.org) format (since 1.0.21), so any age tool opens it. The
lock goes on the copy that leaves the machine; inside the machine, FileVault already
covers the disk and the knowledge base itself stays as it is. The passphrase is typed
twice on the terminal, unechoed, eight characters at least; a script sets
`GTMUX_HQ_PASSPHRASE`, an app pipes it as the first line of stdin with
`--passphrase-stdin`, never on the command line, where `ps` would show it. Lose the
passphrase and the file stays shut: gtmux keeps no copy. `--plain` writes the unlocked
tar.gz. `--import` tells the two apart by the file's header and asks for the passphrase
when it needs one; a wrong passphrase changes nothing. `--records` adds when the last
export was made and whether it was locked; the menu-bar reader asks for the passphrase
in a sheet that can keep it in the Mac's keychain.

`--import` never overwrites in place: existing records are moved to
`hq.replaced-<timestamp>` and the path is printed.

`gtmux serve` snapshots daily to `~/.local/share/gtmux/hq-snapshots/`, keeping 14, and
only writes when the records actually changed. Snapshots live beside the state, outside
the HQ home.

**Snapshots cover accidents, not the disk.** They sit on the same one. The `HQ records`
row in `gtmux doctor` says how much is at risk, how long it took to accumulate, and
whether anything at all carries it off this disk.

## Permissions

gtmux asks for only what it needs:

- Automation (control your terminal: Ghostty / iTerm2 / Warp), required for `focus` /
  `restore` / `new` and notification click-to-jump. macOS prompts the first time gtmux
  drives the terminal via AppleScript; click Allow.
- Notifications, so the menu-bar app can post agent banners. Allow on first run.
- Launch at login (optional), only if you enable it in Preferences.

It does not need these; if macOS prompts, you can safely Deny with no loss of function:

- App Management ("modify apps on your Mac"). gtmux never modifies other apps; its code
  only ever touches its own bundle (on update/uninstall). The prompt can appear when
  macOS attributes another app's self-update to gtmux's long-running background process
  via its responsible-process chain. Denying changes nothing for gtmux.
- Files & Folders (Downloads / Desktop / Documents). gtmux doesn't read these. The
  prompt can appear when `restore` recreates a tmux session whose working directory
  lives in one of them; that's `tmux` (run by gtmux) opening the folder. Safe to deny.
