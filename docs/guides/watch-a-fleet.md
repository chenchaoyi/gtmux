---
title: Monitor coding agents in tmux
description: Start coding agents in tmux, configure status reporting and open sessions that need a reply.
order: 1
---

**English** · [中文](watch-a-fleet.zh.md)

After [installation and doctor](../../README.md#get-connected), use this guide to run agents in tmux, inspect their status and restore sessions.

## Run each agent in its own tmux pane

Give each agent its own tmux pane so gtmux can focus it and send replies. Hooks can also provide read-only status for agents outside tmux. After installing and signing in to the agent CLIs, create a named session:

```sh
tmux new -s api 'claude'
```

Detach with `Ctrl-b d`, or open another terminal window, before starting the next one:

```sh
tmux new -s docs 'codex'
```

A tmux session and its agents can keep running when you close the terminal, lose SSH or unplug a display. Session names and pane IDs help you locate each agent.

After a reboot, `gtmux restore` rebuilds the sessions and layout from the latest tmux-resurrect snapshot. Programs start as new processes; by default, agents that support conversation recovery resume their saved conversations. Use `gtmux doctor` to configure snapshots.

A MacBook normally sleeps when its lid closes; `gtmux awake on` keeps it running. For tmux basics, see the official [Getting Started](https://github.com/tmux/tmux/wiki/Getting-Started) guide.

## Put your agents on the radar

Install a hook once for each agent you use. It reports turn and approval state directly;
Copilot and Kiro need it to appear on the radar. Some other agents can also be detected
from their process and screen:

```sh
gtmux install hooks --agent claude
gtmux install hooks --agent codex
```

Without `--agent` it sets up Claude Code only. Repeat for the agents you use: `gemini`,
`cursor`, `opencode`, `kimi`, `copilot` or `kiro`. An agent that was already running picks
the hook up after a restart. `gtmux doctor --fix` offers hooks for supported installed agents. You can also use `gtmux install hooks --agent auto`; see [agent setup](agent-integrations.md) for each integration’s loading requirements.

For agents with process detection, gtmux can read fallback state without a hook from
the pane title, screen and CPU sampling, with less detail (a live Codex approval menu still shows as
waiting). Visibility outside tmux depends on each agent’s hooks and history reader.

## Open the radar

```sh
gtmux agents --watch      # a live dashboard in your terminal; ↑/↓ select, Enter jumps
```

Or keep it in view: the menu-bar app (installed by the install script, or
`brew install --cask chenchaoyi/tap/gtmux-app`) shows a status dot, opens a palette with
`⌘⌥G`, and posts a notification when an agent needs you. The iPhone and iPad app shows the
same radar. See [Manage from your phone and the web](phone-and-web.md).

![The terminal radar, rendered from real CLI output with example sessions](img/terminal-agents-en.png)

## See every tmux pane

`gtmux panes` lists all tmux panes, including agents, shells, editors and dev servers. In the apps, open All panes.

![All panes on iPad: every tmux session as a card with its panes, agents marked by state and plain shells beside them](img/watch-a-fleet-panes-en.jpg)

## Jump to the one that is waiting

```sh
gtmux focus %7            # bring that pane's window and pane to the front
```

Clicking an agent in the menu-bar panel or opening its notification also brings its terminal window and pane forward so you can reply. Focus supports Ghostty 1.3+, iTerm2 and cmux; Warp support is best effort. It also requires tmux’s `set-titles` option, which `gtmux doctor` can configure ([`gtmux focus`](../cli.md#gtmux-focus)).

On the Mac, ⌥⌘4 captures an image for an agent. An open menu-bar panel stays in the capture so you can frame the panel itself; Esc cancels and keeps it open. In the editor, Return adds a new line
to the note and ⌘↩ sends. Resizing the window scales the image and its marks together;
the exported screenshot keeps its original pixel size.

---

Related guides: [Coordinate tasks with HQ](hq-supervisor.md) · [Manage from your phone and the web](phone-and-web.md) · [CLI reference](../cli.md#gtmux-agents)
