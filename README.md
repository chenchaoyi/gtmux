# gtmux

**Command center for your tmux sessions and coding agents.**

[中文](README.zh.md)

`gtmux` is a small Go CLI that sits over the tmux sessions you already run and
the coding agents (Claude Code, Codex, Gemini, aider, …) running inside them.
It tells you who's **waiting on you**, who's **working**, and who's **idle** —
and jumps you to the exact Ghostty tab and tmux pane.

**How it's different.** Unlike agent *spawners* (claude-squad, uzi, dmux, …)
that create and sandbox agents in git worktrees, gtmux **doesn't run your
agents** — it's the **radar + remote over whatever you already run in tmux**.
Non-invasive, tmux-native, and it even surfaces agents *other* tools spawned
(they're in tmux too). The "g" is for Go.

> macOS + [Ghostty](https://ghostty.org) 1.3+. `restore`/`focus` drive Ghostty
> via AppleScript; `agents`/`overview` work on any tmux.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

Downloads the signed-by-checksum binary to `~/.local/bin/gtmux`. Pin a version:

```sh
GTMUX_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

### China / unstable GitHub

The installer is **GitHub-first and auto-falls back to a mirror chain**
(`ghfast.top` → `gh-proxy.com` → `ghproxy.net`) when a GitHub asset download
stalls. `SHASUMS256.txt` is always fetched GitHub-direct first, so the checksum
the tarball is verified against stays anchored on GitHub even when the tarball
itself came through a mirror. Override with `GTMUX_INSTALL_MIRROR`:

```sh
# go straight to the mirror chain (you know GitHub is unreachable)
GTMUX_INSTALL_MIRROR=ghproxy curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash

# a custom <prefix><github-url> proxy
GTMUX_INSTALL_MIRROR=https://my.mirror/ curl -fsSL ... | bash

# GitHub only, no mirrors
GTMUX_INSTALL_MIRROR=github curl -fsSL ... | bash
```

### From source

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

## Usage

```
gtmux <command> [options]      # bare gtmux prints help
```

| command | what it does |
| --- | --- |
| `overview [--popup]` | sessions / windows / panes summary; `--popup` fits a tmux popup |
| `agents [--watch\|--json]` | coding agents across your panes: ⏸ waiting / ⠿ working / ✳ idle, where, and the pane id to jump to. `--watch` is a live dashboard; `--json` is structured output |
| `restore [--pick\|--one\|<name>\|--dry-run]` | one Ghostty tab per session, attach all |
| `focus <name\|pane-id>` | jump to a session's Ghostty tab; a pane id (`%N`) lands on that exact pane |

Bare `gtmux` prints help; `gtmux --version` prints the version. Output language
follows `--lang=en|zh` (default `en`) or `$GTMUX_LANG`. It's invoked explicitly —
no shell hooks, works with any shell.

## `gtmux agents`

```
gtmux agents — 6 agents · 1 waiting · 1 working · 4 idle

⏸ waiting  Claude Code  Pica:0.0          permission to run tests   %7
⠿ working  Claude Code  ccy-workspace:0.0 Auto-attach tmux sessions %11
✳ idle     Claude Code  Rodi:0.0          Rodi feature dev   %8  ✓ latest
✳ idle     Claude Code  Diting:0.0        —                  %1

jump: gtmux focus <pane>   (e.g. gtmux focus %11)
```

One place to see who's working, who's idle, and who just finished. Each row:
**status**, the **agent**, location, the task, and the **pane id** — sorted by
urgency (waiting → working → idle), with a breakdown in the header. The three
states:

- **⠿ working** — busy (don't bother it)
- **⏸ waiting** — blocked on **you** for a permission/approval, mid-task; sorts to
  the very top so you instantly see which agent needs a decision
- **✳ idle** — finished its turn, your move when ready (not urgent)

**`gtmux agents --watch`** is a live, auto-refreshing dashboard (built with
[bubbletea](https://github.com/charmbracelet/bubbletea)): polls ~1.5s, **↑/↓**
select, **Enter** jumps to the pane, **r** refresh, **q** quit. Agents that
finish while you watch (working → idle) get flagged `✓ done`. **`--json`** emits
the same data as a structured array for scripts/menu-bar apps.

**Detection is not Claude-only:**
- **Status** comes from the pane title the agent sets itself. A leading braille
  spinner (`⠋⠙⠹…`, what most agent TUIs animate) = **working**; Claude Code's `✳`
  = **idle**. This generalizes across agents that animate a spinner.
- **Which agent** is matched by foreground command (`claude`, `codex`, `gemini`,
  `aider`, `opencode`, …) or by a name in the title.
- Extend/override via **`~/.config/gtmux/agents.json`** — a JSON array of
  `{"name","commands","idleGlyph"}`; your entries win over the built-ins.
- A pane is listed only if the agent **process is actually running** (foreground
  command is the agent, or the title is animating a spinner). A leftover agent
  title over a plain shell — e.g. a tmux-resurrect-restored session where the
  agent was never relaunched — is **not** counted.

> `⏸ waiting` and `✓ latest` come from state files under
> `~/.local/share/gtmux/` (`waiting/<pane>`, `last-finished`) written by a
> notification hook — the [reference producer](#notification-hook) is the
> `claude-notify` hook, which tells a *permission* request from an idle nudge by
> hook-event **timing**, not message keywords. Without that hook, agents never
> show `⏸`; everything else still works.

## `gtmux restore`

Quitting Ghostty leaves the tmux server and all sessions alive — only the tabs
are gone. After reopening Ghostty, run **once** in any tab:

```sh
gtmux restore            # one Ghostty tab per tmux session, all attached
```

It opens one tab per session (via Ghostty 1.3+ AppleScript) and attaches them
all; the tab you ran it in takes the first session. The first run pops an
Automation permission dialog ("wants to control Ghostty") — click Allow. Tabs
are created in session-name order; the original tab↔session arrangement isn't
recorded, so it can't be reproduced exactly. Per-tab control:

```sh
gtmux restore --pick     # list sessions (windows & status), choose: "1 3" / "1,3",
                         # Enter = all detached, q = cancel
gtmux restore --one      # attach the next unattached session here
gtmux restore <name>     # attach a specific session by name here
gtmux restore --dry-run  # print what would happen, change nothing
```

**After a reboot** the tmux server itself is gone. `gtmux restore` still works:
it starts tmux and waits for [tmux-continuum](https://github.com/tmux-plugins/tmux-continuum)
to restore the last autosave — sessions, windows, per-pane directories, on-screen
text. **Running programs are not restarted**; each pane comes back as a shell in
its old directory (e.g. relaunch Claude Code with `claude --resume`). Requires
tmux-resurrect/continuum installed.

## `gtmux overview`

```
gtmux overview — 2 sessions · 3 windows · 5 panes

▶ ccy-workspace        1 window · 1 pane
    0: ccy-workspace *  (1 pane)
● Pica                 2 windows · 4 panes
    0: editor  (1 pane)
    1: claude *  (3 panes)

▶ current  ● attached  ○ detached   * active  Z zoomed  • new output
```

A sessions/windows/panes summary from any shell. **`gtmux overview --popup`** is
size-fitted for a tmux `display-popup`, so you can bind it to a key and float it
over a full-screen program without interrupting it (see [tmux integration](#tmux-integration)).

## `gtmux focus`

```sh
gtmux focus Pica         # bring the Ghostty tab showing session "Pica" to front
gtmux focus %11          # jump to that exact window+pane, then focus its tab
```

This is the read side of tmux's `set-titles`: because each tab title is
`session — window`, `focus` finds the matching tab and runs Ghostty's AppleScript
`select tab` + `activate`. A pane id (`%N`) additionally `select-window` +
`select-pane`s inside the session first, so you land on the exact pane — which is
how a notification click can drop you on the agent that just finished.

> Needs `set-titles on` with `set-titles-string '#S — #W'` (so tab titles stay
> in the format `focus` matches). If another tool also writes the tab title,
> disable that so titles stay authoritative.

## menu-bar app

gtmux has two faces over one source of truth: the **CLI** (in the terminal) and a
**menu-bar app** (always visible). The app is an `LSUIElement` status item — your
ambient radar over coding agents — showing at a glance how many are **⏸ waiting on
you / ⠿ working / ✳ idle**, with a dropdown to jump to any of them.

```sh
gtmux install-app            # build/register Gtmux.app and launch it
gtmux install-app --login    # …and start it at login
gtmux uninstall-app          # remove it (and the login item)
```

The status item is a colored dot for the most-urgent state — **red** waiting ·
**cyan** working · **green** idle · gray when nothing's running — with a count
badge (e.g. `2` when two agents need you). The dropdown lists each agent
`‹glyph› session · task`; clicking a row runs `gtmux focus <pane>` to land you on
it, and a **Waiting only** toggle filters to just the ones blocking you. It's a
pure **consumer** of the CLI — it polls `gtmux agents --json` (~1.5s), shells out
to `gtmux focus`, and watches `~/.local/share/gtmux/` so a hook firing (an agent
starting to wait, or finishing) updates the bar instantly. Chrome follows
`GTMUX_LANG` (en/zh).

The menu also has quick actions: **Overview** and **Live watch** (the full
`gtmux overview` / `agents --watch` views, each in a fresh Ghostty window),
**Restore detached** (`gtmux restore`), and **New session** (`gtmux new` — create
a tmux session and open a tab for it). `gtmux new [name]` is a CLI command too.

It's a separate app from the notification click target (`GtmuxFocus.app`,
`com.gtmux.focus`); the two coexist. The app is cgo (Cocoa via `energye/systray`),
so it ships as a separate universal `Gtmux.app` — the CLI binary stays cgo-free.
Build from source with `make app`.

> Releases attach a `Gtmux-<version>-macos.zip` (ad-hoc signed). On first launch
> macOS may warn about an unsigned app; the installer strips the quarantine flag.

## tmux integration

gtmux is just a CLI — bind whatever keys you like in `tmux.conf`. Suggested:

```tmux
set -g set-titles on
set -g set-titles-string '#S — #W'
bind g run-shell -b "gtmux overview --popup"
bind a display-popup -E -w 80% -h 60% "gtmux agents --watch --popup"
bind J run-shell "gtmux focus --last"
```

## notification hook

`⏸ waiting`, `✓ latest`, and click-to-jump notifications rely on a hook writing
state files under `~/.local/share/gtmux/`. gtmux ships that hook built in — no
external script needed:

```sh
gtmux install-hooks          # one-time setup (macOS)
gtmux uninstall-hooks        # reverse it
```

`install-hooks` registers `gtmux hook` in `~/.claude/settings.json` on the
`Stop`, `Notification`, and `UserPromptSubmit` events (idempotent; it preserves
any other hooks and backs the file up first), generates `~/Applications/
GtmuxFocus.app` (bundle id `com.gtmux.focus`) as the notification's click target,
and caches the Claude icon. `terminal-notifier` makes the banner clickable
(`brew install terminal-notifier`); without it, notifications still fire but
aren't clickable.

`gtmux hook` is the producer — Claude Code runs it; you don't. It writes
`active/<pane>`, `waiting/<pane>`, and `last-finished` purely by event timing,
and suppresses the banner when you're already looking at that session's tab. A
click opens `GtmuxFocus.app`, which runs `gtmux focus --last` to land you on the
pane that just finished. Set `GTMUX_HOOK_DEBUG=1` to trace decisions to
`~/.local/share/gtmux/hook.log`.

> Using peon-ping? `install-hooks` offers to set its `desktop_notifications` and
> `terminal_tab_title` to `false` (the latter is required — `focus` needs tmux's
> `set-titles` to own the tab title). Pass `--yes` to accept non-interactively.

## License

MIT
