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
go install github.com/chenchaoyi/gtmux@latest
```

## Usage

```
gtmux <command> [options]      # bare gtmux prints help
```

| command | what it does |
| --- | --- |
| `overview [--popup]` | sessions / windows / panes summary (the prefix+g popup) |
| `agents [--watch\|--json]` | coding agents across your panes: ⏸ waiting / ⠿ working / ✳ idle, where, and the pane id to jump to. `--watch` is a live bubbletea dashboard; `--json` is structured output for scripts |
| `restore [--pick\|--one\|<name>\|--dry-run]` | one Ghostty tab per session, attach all (boots tmux + waits for continuum after a reboot) |
| `focus <name\|pane-id>` | jump to a session's Ghostty tab; a tmux pane id (`%N`) lands on that exact window+pane |

Output language follows `--lang=en|zh` (default `en`) or `$GTMUX_LANG`.

### agent status

- **⏸ waiting** — blocked on **you** for a permission/approval (sorts to the top)
- **⠿ working** — busy (a spinner is animating in the pane)
- **✳ idle** — finished its turn; your move

`waiting` needs the Claude Code notification hook to be installed (it
distinguishes a permission prompt from an idle nudge by event timing, not
keywords). Agents are detected by foreground command and pane-title signals;
extend the profiles via `~/.config/gtmux/agents.json`.

## License

MIT
