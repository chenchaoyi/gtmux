<div align="center">

<img src="docs/assets/logo.png" width="112" alt="gtmux logo" />

# gtmux

**Command center for your tmux sessions and coding agents.**

[![Release](https://img.shields.io/github/v/release/chenchaoyi/gtmux?color=06B6D4&label=release)](https://github.com/chenchaoyi/gtmux/releases)
[![CI](https://github.com/chenchaoyi/gtmux/actions/workflows/ci.yml/badge.svg)](https://github.com/chenchaoyi/gtmux/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.24-00ADD8?logo=go&logoColor=white)](go.mod)
[![Platform](https://img.shields.io/badge/macOS-Ghostty%201.3%2B%20%7C%20iTerm2-111)](https://ghostty.org)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**English** · [中文](README.zh.md)

<img src="docs/assets/screenshot-popover.png" width="380" alt="gtmux menu-bar popover — agents grouped by who needs you, who's working, who's idle" />

</div>

---

`gtmux` sits over the tmux sessions you already run and the coding agents
(Claude Code, Codex, Gemini, aider, …) running inside them. At a glance it tells
you who's **waiting on you**, who's **working**, and who's **idle** — then jumps
you to the exact terminal tab and tmux pane in one click.

Two faces over one source of truth: a small **Go CLI** in your terminal, and an
always-visible **macOS menu-bar app**.

### Why it's different

Unlike agent *spawners* (claude-squad, uzi, dmux, …) that create and sandbox
agents in git worktrees, gtmux **doesn't run your agents** — it's the
**radar + remote over whatever you already run in tmux**. Non-invasive,
tmux-native, and it even surfaces agents *other* tools spawned (they're in tmux
too). The "g" is for Go.

### Scope

gtmux is **focused on the tmux + agent workflow**: it tracks coding agents
running **inside tmux**. Agents started **directly in a terminal tab (no tmux)
are not detected** — that's a deliberate focus, not a bug. Supporting native,
non-tmux terminals is a possible future direction; for now, run your agents in
tmux to see them.

**Supported terminals.** The radar (`agents` / `overview` / notifications) is
terminal-agnostic — it works under **any** terminal that hosts tmux. The jump
side (`focus` / `restore` / `new`) drives the terminal via AppleScript and
currently supports **Ghostty** (1.3+) and **iTerm2**; the host terminal is
auto-detected (override with `GTMUX_TERMINAL`). Other AppleScript-/CLI-scriptable
terminals (Apple Terminal, kitty, WezTerm) are feasible to add on request. **Warp
and Alacritty are not supported** — they don't expose the tab-addressing
automation gtmux needs.

### Highlights

- 🛰️ **One glance, every agent** — `⏸ waiting · ⠿ working · ✳ idle`, sorted by urgency.
- 🎯 **One-click jump** — land on the exact Ghostty tab **and** tmux pane that needs you.
- 🔔 **Knows when *you're* needed** — a built-in hook tells a permission prompt from an idle nudge by event *timing*, not keyword guessing.
- 🍫 **Menu-bar app** — a native, ambient status dot (red/cyan/green) with a popover and a ⌘⌥G command palette.
- 🧩 **Agent-agnostic** — detects any agent that animates a spinner; extend via one JSON file.
- 🪶 **Non-invasive & cgo-free** — reads tmux, never owns your agents; one static Go binary.

> **Requires** macOS + [Ghostty](https://ghostty.org) 1.3+ **or** iTerm2.
> `restore`/`focus`/`new` drive the host terminal via AppleScript (auto-detected;
> override with `GTMUX_TERMINAL`); `agents`/`overview` work on any tmux.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

Installs the checksum-verified binary to `~/.local/bin/gtmux`, plus the menu-bar
app (`GTMUX_NO_APP=1` to skip · `GTMUX_APP_LOGIN=1` to start it at login). Pin a
version with `GTMUX_VERSION=vX.Y.Z`. From source: `go install
github.com/chenchaoyi/gtmux/cmd/gtmux@latest`.

<details>
<summary>China / unstable GitHub — mirror fallback</summary>

The installer is **GitHub-first and auto-falls back to a mirror chain**
(`ghfast.top` → `gh-proxy.com` → `ghproxy.net`) when a GitHub asset download
stalls. `SHASUMS256.txt` is always fetched GitHub-direct first, so the checksum
stays anchored on GitHub even when the tarball came through a mirror. Override
with `GTMUX_INSTALL_MIRROR`:

```sh
GTMUX_INSTALL_MIRROR=ghproxy  curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash   # straight to the mirror chain
GTMUX_INSTALL_MIRROR=https://my.mirror/  curl -fsSL ... | bash   # custom <prefix><github-url> proxy
GTMUX_INSTALL_MIRROR=github   curl -fsSL ... | bash   # GitHub only, no mirrors
```

</details>

## At a glance — `gtmux agents`

```
gtmux agents — 6 agents · 1 waiting · 1 working · 4 idle

⏸ waiting  Claude Code  Pica:0.0          permission to run tests   %7
⠿ working  Claude Code  ccy-workspace:0.0 Auto-attach tmux sessions %11
✳ idle     Claude Code  Rodi:0.0          Rodi feature dev   %8  ✓ latest
✳ idle     Claude Code  Diting:0.0        —                  %1

jump: gtmux focus <pane>   (e.g. gtmux focus %11)
```

One place to see who's working, who's idle, and who just finished. Each row is
**status · agent · location · task · pane id**, sorted by urgency. The three
states:

- **⠿ working** — busy (don't bother it).
- **⏸ waiting** — blocked on **you** for a permission/approval, mid-task; sorts to
  the very top so you instantly see which agent needs a decision.
- **✳ idle** — finished its turn, your move when ready (not urgent).

**`gtmux agents --watch`** is a live, auto-refreshing dashboard (built with
[bubbletea](https://github.com/charmbracelet/bubbletea)): polls ~1.5s, **↑/↓**
select, **Enter** jumps to the pane, **r** refresh, **q** quit. **`--json`**
emits the same data for scripts and the menu-bar app.

<details>
<summary>How detection works (not Claude-only)</summary>

- **Status** comes from the pane title the agent sets itself. A leading braille
  spinner (`⠋⠙⠹…`, what most agent TUIs animate) = **working**; Claude Code's `✳`
  = **idle**. This generalizes across agents that animate a spinner.
- **Which agent** is matched by foreground command (`claude`, `codex`, `gemini`,
  `aider`, `opencode`, …) or by a name in the title.
- Extend/override via **`~/.config/gtmux/agents.json`** — a JSON array of
  `{"name","commands","idleGlyph"}`; your entries win over the built-ins.
- A pane is listed only if the agent **process is actually running**. A leftover
  agent title over a plain shell (e.g. a resurrect-restored session never
  relaunched) is **not** counted.

`⏸ waiting` and `✓ latest` come from state files written by the
[notification hook](#notification-hook). Without it, agents never show `⏸`;
everything else still works.

</details>

## Menu-bar app

<div align="center">
<img src="docs/assets/screenshot-popover.png" width="340" alt="popover" />
&nbsp;&nbsp;
<img src="docs/assets/screenshot-firstrun.png" width="340" alt="first-run permission card" />
</div>

Your ambient radar over coding agents — a native macOS `LSUIElement` status item
(Swift / AppKit). The dot is a colored summary of the most-urgent state —
**red** waiting · **cyan** working · **green** idle · gray when nothing's running
— with a count badge (e.g. `2` when two agents need you).

- **Click the dot, or press ⌘⌥G**, to open the popover / command palette.
- Agents are grouped **Needs you → Working → Idle**; each row is `‹glyph› session · task`.
- **Click a row** (or `⏎` / `⌘1–9`) to run `gtmux focus <pane>` and land on it.
- A footer has **Overview**, **Live watch**, **Restore detached**, and **New session**.

It's a pure **consumer** of the CLI — it polls `gtmux agents --json` and shells
out to `gtmux focus`, so gtmux-core stays the single data source. The CLI stays
cgo-free; the app is the only native build. Releases attach a universal,
ad-hoc-signed `Gtmux-<version>-macos.zip`; the installer strips the quarantine
flag so first launch isn't blocked. Remove it with `gtmux uninstall-app`.

## Commands

| command | what it does |
| --- | --- |
| `agents [--watch\|--json]` | coding agents across your panes: who's waiting / working / idle, where, and the pane id to jump to |
| `overview [--popup]` | sessions / windows / panes summary; `--popup` fits a tmux popup |
| `restore [--pick\|--one\|<name>\|--dry-run]` | one Ghostty tab per session, attach all |
| `focus <name\|pane-id>` | jump to a session's tab; a pane id (`%N`) lands on that exact pane |
| `new [name]` | start a new tmux session in a fresh Ghostty tab |

Bare `gtmux` prints help; `gtmux --version` prints the version. Output language
follows `--lang=en|zh` (default `en`) or `$GTMUX_LANG`. Invoked explicitly — no
shell hooks, works with any shell.

### `gtmux restore`

Quitting Ghostty leaves the tmux server and all sessions alive — only the tabs
are gone. After reopening Ghostty, run **once** in any tab:

```sh
gtmux restore            # one Ghostty tab per tmux session, all attached
gtmux restore --pick     # choose which sessions: "1 3" / "1,3", Enter = all, q = cancel
gtmux restore --one      # attach the next unattached session in this tab
gtmux restore <name>     # attach a specific session here
gtmux restore --dry-run  # print what would happen, change nothing
```

The first run pops an Automation permission dialog ("wants to control Ghostty") —
click Allow. **After a reboot** the tmux server is gone too; `gtmux restore`
starts tmux and **explicitly drives
[tmux-resurrect](https://github.com/tmux-plugins/tmux-resurrect) to restore the
last autosave** (it waits for the restore to finish — large layouts take 30s+ —
and if a saved layout exists but can't be restored it refuses to overwrite it).
Running programs are not restarted — relaunch e.g. with `claude --resume`.

**Each pane's previous output (scrollback) comes back too** — a snapshot — when
resurrect is set to capture it. Recommended in `tmux.conf`:

```tmux
set -g @resurrect-capture-pane-contents 'on'   # snapshot each pane's scrollback
set -g history-limit 50000                     # how much scrollback to keep/restore
```

> The shell's **↑ command history** is separate — it lives in your shell's
> histfile, not in resurrect. By default it's written only on shell exit, so a
> reboot loses recent commands. To persist it immediately (bash):
> `shopt -s histappend; PROMPT_COMMAND='history -a'` in `~/.bashrc` (zsh:
> `setopt INC_APPEND_HISTORY`). The restored scrollback still *shows* past
> commands; this just keeps them recallable with ↑.

### `gtmux overview`

```
gtmux overview — 2 sessions · 3 windows · 5 panes

▶ ccy-workspace        1 window · 1 pane
    0: ccy-workspace *  (1 pane)
● Pica                 2 windows · 4 panes
    0: editor  (1 pane)
    1: claude *  (3 panes)

▶ current  ● attached  ○ detached   * active  Z zoomed  • new output
```

A sessions/windows/panes summary from any shell. **`--popup`** is size-fitted for
a tmux `display-popup`, so you can bind it to a key and float it over a
full-screen program without interrupting it.

### `gtmux focus`

```sh
gtmux focus Pica         # bring the Ghostty tab showing session "Pica" to front
gtmux focus %11          # jump to that exact window+pane, then focus its tab
```

Because each tab title is `session — window`, `focus` finds the matching tab and
runs Ghostty's AppleScript `select tab` + `activate`. A pane id (`%N`) also
`select-window` + `select-pane`s inside the session, so you land on the exact
pane — which is how a notification click drops you on the agent that just
finished.

> Needs `set-titles on` with `set-titles-string '#S — #W'` so tab titles stay in
> the format `focus` matches. If another tool also writes the tab title, disable
> that so titles stay authoritative.

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
state files under `~/.local/share/gtmux/`. gtmux ships that hook **built in** — no
external script needed:

```sh
gtmux install-hooks          # one-time setup (macOS)
gtmux uninstall-hooks        # reverse it
```

`install-hooks` registers `gtmux hook` in `~/.claude/settings.json` on the
`Stop`, `Notification`, and `UserPromptSubmit` events (idempotent; preserves
other hooks and backs the file up). `gtmux hook` is the producer — Claude Code
runs it, you don't — and writes state purely by event **timing**, telling a
permission request from an idle nudge without reading message text.

**Notifications are delivered by the menu-bar app** — no `terminal-notifier`
needed. The hook queues a request under `~/.local/share/gtmux/notify/` and
`Gtmux.app` posts a native banner (shown as **Gtmux**, with the agent icon, a
**Jump** action, and differentiated copy — *finished* is calm and silent,
*needs your input* sounds). Clicking it lands you on the exact pane. Grant
"Allow Notifications" on first run; keep the app running to receive them.

## Permissions

gtmux asks for only what it needs:

- **Automation (control Ghostty)** — required for `focus` / `restore` / `new` and
  notification click-to-jump. macOS prompts the first time gtmux drives Ghostty
  via AppleScript; click **Allow**.
- **Notifications** — so the menu-bar app can post agent banners. Allow on first run.
- **Launch at login** *(optional)* — only if you enable it in Preferences.

It does **not** need these — if macOS prompts, you can safely **Deny** with no
loss of function:

- **App Management ("modify apps on your Mac")** — gtmux never modifies other
  apps; its code only ever touches its own bundle (on update/uninstall). If you
  see this prompt, macOS attributed *another* app's self-update (e.g. a browser
  updating itself) to gtmux's long-running background process via its
  responsible-process chain. Denying changes nothing for gtmux.
- **Files & Folders (Downloads / Desktop / Documents)** — gtmux doesn't read
  these. The prompt can appear when `restore` recreates a tmux session whose
  working directory lives in one of them — that's `tmux` (run by gtmux) opening
  the folder. Safe to deny; only that one session's directory is affected.

> macOS ties granted permissions to the app's code signature. A **Developer
> ID-signed + notarized** build keeps your grants across updates; an **ad-hoc**
> build (a local `make app`, or an unsigned release) changes identity every
> build, so macOS forgets and re-prompts. Set `GTMUX_SIGN_ID` when building to
> sign with your Developer ID (see `macapp/build.sh`).

## License

[MIT](LICENSE) © ccy
