# CLI & commands

| command | what it does |
| --- | --- |
| `agents [--watch\|--json]` | coding agents across your panes: who's waiting / working / idle, where, and the pane id to jump to |
| `overview [--popup]` | sessions / windows / panes summary; `--popup` fits a tmux popup |
| `restore [--pick\|--one\|<name>\|--dry-run]` | one terminal tab per session, attach all |
| `focus <name\|pane-id>` | jump to a session's tab; a pane id (`%N`) lands on that exact pane |
| `new [name]` | start a new tmux session in a fresh terminal tab |

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

`⏸ waiting` and `✓ latest` come from state files written by the
[notification hook](#notification-hook). Without it, agents never show `⏸`;
everything else still works.

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
gtmux install-hooks          # one-time setup (macOS)
gtmux uninstall-hooks        # reverse it
```

`install-hooks` registers `gtmux hook` in `~/.claude/settings.json` on the
`Stop`, `Notification`, and `UserPromptSubmit` events (idempotent; preserves
other hooks and backs the file up). `gtmux hook` is the producer — Claude Code
runs it, you don't — and writes state purely by event **timing**, telling a
permission request from an idle nudge without reading message text.

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
