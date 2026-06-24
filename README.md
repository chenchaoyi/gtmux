<div align="center">

<img src="docs/assets/logo.png" width="104" alt="gtmux logo" />

# gtmux

**See which coding agent needs you across your tmux sessions — jump to the exact pane, reply, and get a push the moment one's blocked. From your terminal, the menu bar, or your phone.**

[![Release](https://img.shields.io/github/v/release/chenchaoyi/gtmux?color=06B6D4&label=release)](https://github.com/chenchaoyi/gtmux/releases)
[![CI](https://github.com/chenchaoyi/gtmux/actions/workflows/ci.yml/badge.svg)](https://github.com/chenchaoyi/gtmux/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.25-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

**English** · [中文](README.zh.md)

</div>

---

You run coding agents — Claude Code, Codex, Gemini, aider — inside tmux, often
several at once. They go quiet, and you lose track of which one is waiting on a
yes/no, which is still working, and which just finished.

gtmux is the radar over them. It reads the agents in your tmux, shows who needs
you, and jumps you to the exact pane. When you step away, it tells you the moment
an agent needs a decision — in the menu bar, on the desktop, or on your phone.

It does **not** run your agents. It watches whatever you already have in tmux —
including agents other tools started — and never gets in the way.

**Three surfaces, one source of truth:**

- **CLI** — `gtmux agents` lists every agent and where to jump; `--watch` is a live dashboard.
- **Menu-bar app** — an always-visible status dot (red / cyan / green) with a popover and a `⌘⌥G` palette.
- **Mobile app** — the same radar on iOS, with a lock-screen push when an agent needs you, and a tap to reply.

<div align="center">
<img src="docs/assets/surface-cli.png" width="252" alt="CLI — gtmux agents" />
<img src="docs/assets/surface-menubar.png" width="252" alt="Menu-bar app — popover + status dot" />
<img src="docs/assets/surface-mobile.png" width="252" alt="Mobile app — the agent radar on iOS" />
</div>

## When you'd use it

- You're running several agents and keep alt-tabbing to check which is blocked.
- You stepped away and want a nudge the moment one needs a yes/no — not ten minutes later.
- You're away from the Mac (home, office, commute) and want to check or unblock an agent from your phone.
- Your Mac rebooted and you want your tmux sessions and tabs back in one command.

## At a glance — `gtmux agents`

```
gtmux agents — 6 agents · 1 waiting · 1 working · 4 idle

⏸ waiting  Claude Code  Pica:0.0          permission to run tests    %7
⠿ working  Claude Code  ccy-workspace:0.0 Auto-attach tmux sessions  %11
✳ idle     Claude Code  Rodi:0.0          Rodi feature dev    %8  ✓ latest
✳ idle     Claude Code  Diting:0.0        —                   %1

jump: gtmux focus %7
```

Each row is **status · agent · location · task · pane id**, sorted by urgency:

- **⏸ waiting** — blocked on **you** mid-task (a permission/approval). Sorts to the top.
- **⠿ working** — busy; leave it alone.
- **✳ idle** — finished its turn; your move when ready.

Detection is by event *timing*, not keyword guessing, and works with any agent
that animates a spinner — not just Claude Code.

## Quickstart

Homebrew (macOS):

```sh
brew install chenchaoyi/tap/gtmux             # the CLI
brew install --cask chenchaoyi/tap/gtmux-app  # the menu-bar app (optional)
```

Or the install script (CLI + menu-bar app in one shot):

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

Installs the CLI to `~/.local/bin/gtmux` and the menu-bar app. Then:

```sh
gtmux install-hooks          # so agents can report "waiting on you" (one-time)
gtmux agents --watch         # the live dashboard; Enter jumps to a pane
```

To watch from your phone, run `gtmux serve` (same network) or `gtmux tunnel`
(anywhere) and pair the iOS app. See **[docs/phone.md](docs/phone.md)**.

> **Requires** macOS + [Ghostty](https://ghostty.org) 1.3+ **or** iTerm2 for the
> jump features (`focus` / `restore` / `new`); `agents` / `overview` work under
> any terminal that hosts tmux. Mainland China / unstable GitHub: see the
> [install notes](docs/install.md).

## Docs

- **[CLI & commands](docs/cli.md)** — `agents` / `overview` / `focus` / `restore` / `new`, detection, the notification hook, tmux key bindings, and permissions.
- **[Mobile & remote access](docs/phone.md)** — the iOS app, `gtmux serve`, and reaching your Mac from anywhere with Tailscale or `gtmux tunnel`.
- **[Install notes](docs/install.md)** — pinning a version, building from source, and the China / mirror fallback.
- **Design specs** — `docs/design/` (menu-bar `DESIGN.md`, mobile `MOBILE.md`) and `openspec/` for in-flight changes.

## How it's different

Tools like claude-squad, uzi, and dmux *spawn* agents and sandbox them in git
worktrees. gtmux is the opposite: it runs nothing, owns nothing, and is just a
radar plus a remote over the tmux you already use. One static, cgo-free Go
binary; the menu-bar and mobile apps are pure consumers of the same `gtmux agents
--json`. The "g" is for Go.

## License

[MIT](LICENSE) © ccy
