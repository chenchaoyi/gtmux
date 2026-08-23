# tmux-server-own-identity — your agents answer for themselves

## Why

macOS asks "**Gtmux.app** would like to access data from other apps" while gtmux is
doing nothing at all. What is actually happening is one of the user's own agents doing
its work — reading another app's cache while clearing disk space, say — and the prompt
is putting gtmux's name on it.

A background app that repeatedly asks for other apps' data looks like an app that is
helping itself to them. gtmux is not, and it should not appear to be.

## What is actually going on (measured, not inferred)

macOS attributes a file access to the **responsible process** — the app at the root of
the process tree — and that attribution is fixed when a process is created and never
changes afterwards. Captured live:

    ls ~/Library/Containers/com.apple.podcasts/Data
    └─ bash                        (the agent's tool shell)
       └─ claude --resume …
          └─ -bash
             └─ tmux -u new-session -d -s gtmux-boot-15663      ppid=1

    responsible_path = …/Gtmux.app/Contents/MacOS/GtmuxBar
    binary_path      = /bin/ls

The root is the boot session `gtmux restore` creates (`restore.go`), and that restore was
run from the menu-bar app. So the tmux SERVER was created as the app's child, inherited
the app's identity, and kept it after launchd adopted it (`ppid=1` hides this completely
— nothing in `ps` says where a process came from).

From then on **every pane in that server, and every command every agent runs in it,
answers to Gtmux.app** — until the server is restarted. It is not intermittent; it is
permanent for that server, and shows up whenever an agent's work happens to touch
another app's data.

## What changes

Start the tmux server through launchd rather than as gtmux's own child, so it gets its
own identity. Measured on this machine, same read, two ways of starting the server:

    started directly   →  responsible = GtmuxBar     ("Gtmux.app would like to…")
    started by launchd →  responsible = tmux

`setsid` was tried first and does nothing here — responsibility is not the process group
or the parent, and survives both.

**Only on the path that actually misattributes.** A restore run from a terminal already
gets this right: the server is the terminal's child, and the terminal IS the app the user
launched, so its name on a prompt is the honest answer. The menu-bar app is the only
caller whose name is the wrong one, and it is also the only caller that KNOWS it is an
app — gtmux cannot read its own responsible process without root. So the app asks for the
detour explicitly, and every other caller keeps today's behaviour untouched:

| caller | today | after |
|---|---|---|
| menu-bar app | prompt says Gtmux.app ✗ | prompt says tmux ✓ |
| a terminal | prompt says the terminal ✓ | unchanged |
| Linux, or launchd unavailable | — | unchanged (fallback) |

## The environment is not optional here

A launchd-started process does not inherit the user's environment, and this repo has
already paid for that once: a launchd-started `serve` had no `LANG`, tmux mangled UTF-8,
and the radar reported zero agents on a machine full of them (v0.11.3). Measured again
for this change, a tmux server started by launchd hands its panes:

    PATH=/usr/bin:/bin:/usr/sbin:/sbin      (no Homebrew — no tmux, no git, no agents)
    LANG unset, LC_CTYPE unset

So the environment must be passed EXPLICITLY.

**Correction from implementing it.** The acceptance check was going to be "a CJK pane
title reads back correctly". Two versions of that assertion were written and both passed
with the environment deliberately stripped, so neither tested anything: a non-ASCII
window NAME comes from the client and is only stored by the server, and non-ASCII a pane
PRINTS survives anyway on tmux 3.7 regardless of locale — the v0.11.3 failure was an
older tmux. What is still load-bearing today is PATH: without it the server holds
`/usr/bin:/bin:/usr/sbin:/sbin`, so nothing it starts can find tmux, git or any agent.
The test asserts that, and goes red when the environment is not passed.

## Boundaries

- **It cannot fix a server that is already running.** Identity is set at creation, so an
  existing session keeps answering as Gtmux.app until tmux is next restarted. The fix
  applies to servers created from then on. Saying so is part of shipping it.
- **restore is the most delicate path in the repo** — it is what brings a working set
  back after a reboot, and it has cost real work before (phantom `claude --resume`
  injection, a resurrect archive with shifted fields, a `last` pointing at an empty
  save). The change must not alter what restore restores, only who creates the server.
- If launchd is unavailable or refuses, restore MUST fall back to starting the server
  the way it does today. A working set that comes back under the wrong name is far
  better than one that does not come back.
