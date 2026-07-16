# CLI & commands

| command | what it does |
| --- | --- |
| `agents [--watch\|--json]` | coding agents across your panes: who's waiting / working / idle, where, and the pane id to jump to |
| `overview [--popup]` | sessions / windows / panes summary; `--popup` fits a tmux popup |
| `restore [--pick\|--one\|<name>\|--dry-run] [--resume-agents=auto\|type\|off]` | one terminal tab per session, attach all; optionally relaunch captured agent conversations |
| `focus <name\|pane-id\|--last>` | jump to a session's tab; a pane id (`%N`) lands on that exact pane; `--last` = the most-recently-finished agent |
| `new [name]` | start a new tmux session in a fresh terminal tab |
| `adopt <session_id>…` | move a sensed non-tmux (native) agent session into tmux |
| `doctor [--fix [--yes]]` | health check grouped by concern; `--fix` is the one-stop setup (hook, set-titles, restore, the app) |
| `install-hooks [--agent <key>]` | register the notification hook — Claude by default; `--agent codex\|cursor\|gemini\|copilot\|kiro` for others |
| `serve [--port N]` | read-only HTTP+SSE radar for the mobile app / browser mirror (behind a VPN or tunnel) |
| `tunnel [--backend cloudflare\|self] [--quick] [--service] [--redeem <code>]` | expose the radar from anywhere — Standard (Cloudflare) or Direct (paid); see [phone.md](phone.md) |
| `devices [revoke <id>]` | list / revoke phones paired via per-device tokens |
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
cooperate.

`gtmux hq` opens (or focuses — never duplicates) the **supervisor**: your coding
agent running in a dedicated tmux session at `~/.config/gtmux/hq/`, seeded once
with instructions that teach it the loop — read `gtmux digest --json`, judge,
drill into a pane (`tmux capture-pane`) only when warranted, drive via
`gtmux send`, report to you. The playbook is seeded as `AGENTS.md` (the
cross-agent convention) with `CLAUDE.md` as an `@AGENTS.md` import — so the
supervisor can be ANY CLI agent: `gtmux hq --agent codex` (or `GTMUX_HQ_AGENT`).
Edit `AGENTS.md` to change its policy;
notes it keeps in that directory persist across its sessions. In the radar its
row carries `role:"supervisor"`.

When another agent starts **waiting** and an hq session is live, the hook types
one event line into it — `[gtmux] waiting·permission api:0.0 (%7) — <title>` —
so the supervisor learns of blockers without polling (same dedup as
notifications; never about itself; `"hqNudge": false` in
`~/.config/gtmux/config.json` disables). The channel is two-sided and covers more
than menu waits: `[gtmux] resolved …` when a wait CLEARS (the user answered
in-pane, or the agent resumed — so hq drops any stale chase), `[gtmux] asks … "…"`
when a turn-end REPLY asks a question with no menu (the case a menu-only sensor
misses), `[gtmux] done …` when a dispatched task finishes, and
`[gtmux] reap-suggest … · gtmux reap <id>` when a dispatch looks reclaimable.
Ordinary progress turns stay pull-only on `gtmux events --follow` (hq isn't
flooded). The nudge only informs: gtmux never answers another agent's prompt, never
sends navigation keys into a TUI, and the default policy tells the supervisor to
surface decisions to you, not take them.

## `gtmux spawn` / `gtmux tasks` / `gtmux reap` — verified dispatch

`gtmux spawn <goal>` dispatches new work to a coding agent and confirms it actually
LANDED — the supervisor's (and your) reliable way to start a task without hand-rolled
tmux choreography:

```
gtmux spawn "add a --dry-run flag to the deploy script"
gtmux spawn --worktree feat/dry-run --model opus "add a --dry-run flag"
gtmux spawn --pane %14 "keep going, then run the tests"
gtmux spawn --json "…"   # → {task_id, pane_id, session, delivered, state, evidence}
```

It launches the agent (a fresh detached session by default, or `--pane <id>` to reuse
one, or `--worktree <branch>` to run in an isolated git worktree), **through the
network proxy by construction** (never a bare, un-proxied launch that would 403),
waits for the agent to come up, then delivers the task via a tmux **paste buffer** and
**verifies** it landed. Verification is layered: for a hook-equipped agent (Claude
Code, …) it prefers the deterministic `UserPromptSubmit` event on the #388
session-events stream (no screen-scraping); otherwise it falls back to a hardened,
two-frame screen-read that locates the input box structurally. The ONLY success is a
confirmed landing — a swallowed Enter is re-sent with backoff, a fragment paste is
retried, and a timeout reports `delivered:false` with on-screen evidence, never a
silent success. A queued submission is reported as `state:"queued"`. A **re-send
interlock** refuses an identical payload to the same pane within a window (so a nervous
duplicate `/compact` can't double-fire); `--force` overrides it. Pre-flight checks
(proxy, machine resource, subscription window) are advisory and never block.

`gtmux send <pane> <text>` uses the SAME land-verification by DEFAULT now (returns as
soon as it confirms, so a healthy send stays fast); `--no-verify` opts out, `--force`
overrides the interlock, `--key` and the mobile `POST /api/send` path are unchanged
(the API stays fast).

`gtmux tasks [--json]` is the **dispatch / needs-you ledger**: every task you spawned
with its live status (waiting / done / working / gone), needs-you first.

`gtmux reap <pane|task_id>` safely reclaims a finished dispatch — it runs a safety gate
FIRST (the worktree must be clean and the branch merged) and only then kills the
session, removes the worktree, and deletes the merged branch; when the gate fails it
reports exactly what blocks it and touches nothing (`--abandon` overrides,
`--keep-branch` keeps the branch). `--snooze [--for <dur>]` silences a reap suggestion
for a dispatch you're keeping. When a tracked dispatch looks reclaimable, a live hq
gets a `[gtmux] reap-suggest … · gtmux reap <id>` nudge — reclaim is always
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
as a one-per-layer `[gtmux] usage·warn …` nudge into a live HQ session. `--json`
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

## `gtmux resource` — local machine resource watch

```
disk 40GB free · mem 38% free (warn) · load 0.64×14 cores   ⚠ disk 40GB free
per-agent (RSS · CPU):
  %26    252MB · 9.2%
reclaim candidates (orphans no live agent owns):
  pid 3015  100MB · 0.0%  iOS Simulator runtime (12 procs) [simulator]
    ↳ leftover iOS Simulator runtime — `xcrun simctl shutdown all`
```

Disk (`df`), memory (`memory_pressure -Q` free % + the kernel `kern.memorystatus_vm_pressure_level`
normal/warn/critical tier), CPU (loadavg÷cores). **Per-agent RSS/CPU** by walking
each pane's process tree (isomorphic to token accounting), and **reclaim
candidates** — heavy processes no live pane owns, named with pid + how to reclaim
(a leftover iOS Simulator runtime aggregates into one entry; dev servers/tmux
strays surface individually). Thresholds in `~/.config/gtmux/config.json`'s
`resource` object (diskAmberGB 50 / diskRedGB 15 / loadAmber 1.0 / loadRed 1.5 /
orphanRssMB 300). A resource block rides `GET /api/usage`; the serve tick emits a
`resource·warn` nudge to HQ (single-writer — one per crossing); `gtmux hq`/`new`
warn at a red line before adding load.

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
`limitsWarnPct` marks amber and nudges a live HQ once (`[gtmux] limits·warn …`).
The `limits` block also rides `gtmux usage` and `GET /api/usage`.

## `gtmux restore`

Quitting your terminal leaves the tmux server and all sessions alive — only the
tabs are gone. After reopening, run **once** in any tab:

```sh
gtmux restore            # one terminal tab per tmux session, all attached
gtmux restore --pick     # choose which sessions: "1 3" / "1,3", Enter = all, q = cancel
gtmux restore --one      # attach the next unattached session in this tab
gtmux restore <name>     # attach a specific session here
gtmux restore --dry-run  # print what would happen, change nothing
```

The first run pops an Automation permission dialog ("wants to control Ghostty") —
click Allow. **After a reboot** the tmux server is gone too; `gtmux restore`
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

> Needs `set-titles on` with `set-titles-string '#S — #W'` so tab titles stay in
> the format `focus` matches. If another tool also writes the tab title, disable
> that so titles stay authoritative.

## `gtmux attach` — work in a remote session from another machine's terminal

Where `focus` jumps to a LOCAL tab, `attach` opens a REMOTE pane in your current
terminal (Ghostty / iTerm2 / Terminal) as a raw, interactive passthrough — the local
terminal becomes the remote tmux session, over the same `gtmux serve` surface (a
WebSocket, `GET /api/attach`), honoring the owner/guest token scope.

```sh
# owner — full access with the serve token:
gtmux attach http://<mac>:8765 --token <serve-token> %12

# guest — a scope-restricted share link (from `gtmux share new`, or the menu bar's
# "Shared input" → New link); attach exactly what the host allowed:
gtmux attach 'https://<mac>.example/#t=<token>' %12

gtmux attach <target>            # omit the pane: auto-attach the only one, else pick
gtmux attach <target> --read-only  # watch only, never send input
```

- **`<target>`** is a host (+ `--token`, → **owner**, full) or a `…/#t=<token>` share
  link (→ **guest**, restricted to the host's view/input allowlists — a view-only pane
  is read-only, and a non-viewable pane is refused).
- **`%N`** (optional) is the tmux pane id to attach — it selects the SESSION that pane
  is in. Omit it to auto-attach when there's a single session, or get a chooser.
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

## `gtmux share` — scoped, revocable access for a collaborator

```
gtmux share new --label Alice --view %1,%2 --type %1 --expires 24h
gtmux share set a1b2c3d4 --type %2            # edit ONE link (omitted flags untouched)
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
`expires_at` and never a bare token.

## tmux integration

gtmux is just a CLI — bind whatever keys you like in `tmux.conf`. Suggested:

```tmux
set -g set-titles on
set -g set-titles-string '#S — #W'
bind g run-shell -b "gtmux overview --popup"
bind a display-popup -E -w 80% -h 60% "gtmux agents --watch --popup"
bind J run-shell "gtmux focus --last"
```

## Notification hook

`⏸ waiting`, `✓ latest`, and click-to-jump notifications rely on a hook writing
state files under `~/.local/share/gtmux/`. gtmux ships that hook built in — no
external script needed:

```sh
gtmux install-hooks                 # Claude, one-time setup (macOS)
gtmux install-hooks --agent codex   # or cursor|gemini|copilot|kiro
gtmux uninstall-hooks [--agent …]   # reverse it
```

`install-hooks` registers `gtmux hook` in `~/.claude/settings.json` on the
`Stop`, `Notification`, and `UserPromptSubmit` events (idempotent; preserves
other hooks and backs the file up). `gtmux hook` is the producer — Claude Code
runs it, you don't — and writes state purely by event **timing**, telling a
permission request from an idle nudge without reading message text.

**Other agents:** `--agent codex|cursor|gemini|copilot|kiro` wires that agent's
own hooks file instead. **Codex** uses its additive hooks system
(`~/.codex/hooks.json` + `features.hooks`), so it **coexists with any existing
`notify`** (e.g. computer-use) rather than replacing it. `gtmux doctor --fix`
offers to wire whatever agents it detects.

Notifications are delivered by the menu-bar app — no `terminal-notifier` needed.
The hook queues a request under `~/.local/share/gtmux/notify/` and `Gtmux.app`
posts a native banner (shown as **Gtmux**, with the agent icon and a **Jump**
action; *finished* is calm and silent, *needs your input* sounds). Clicking it
lands you on the exact pane. Grant "Allow Notifications" on first run and keep
the app running to receive them.

## Permissions

gtmux asks for only what it needs:

- **Automation (control Ghostty)** — required for `focus` / `restore` / `new` and
  notification click-to-jump. macOS prompts the first time gtmux drives the
  terminal via AppleScript; click **Allow**.
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
