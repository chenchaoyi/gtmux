---
title: Attach to your Mac from any computer
description: Use gtmux attach to work inside your Mac's tmux session from another computer's terminal, over the local network or a tunnel, and detach without stopping anything.
order: 4
---

**English** · [中文](attach-from-anywhere.zh.md)

`gtmux attach` connects a terminal on another computer to an existing tmux session on your Mac. You can operate the programs in that session, then detach without stopping them.

<a id="open-a-door-on-your-mac"></a>

## Enable access on your Mac

```sh
gtmux serve                # the other computer is on the same local network
gtmux tunnel               # a public HTTPS address, for any other network that allows it
```

On the same network `serve` is enough. Across networks, `tunnel` gives the Mac an
`https://…` address over an outbound tunnel, with no port forwarding and no VPN; the
network you attach from still has to allow that address.
[Remote access reference](../phone.md#connection-methods) covers the
tunnel types.

## Install gtmux on the other computer

Use the [official installer](../install.md) on the other computer too; set `GTMUX_NO_APP=1` for CLI-only installation. The installer selects the matching macOS or Linux archive.

Use an interactive terminal: Ghostty, iTerm2, Terminal, or whatever your Linux desktop has.

## Pair that terminal once

On the Mac, run `gtmux pair`. Among its three forms of the code is a one-line
`gtmux attach '…/#c=<code>'` command: copy it into the other computer's terminal and run it.
The code works once and expires in five minutes, so use a fresh one each time you pair.

The terminal then remembers its credential, and from then on this is enough:

```sh
gtmux attach <host> %7
```

`%7` is the pane ID. Leave it out and gtmux picks from the panes on your radar (agents in
tmux, plus any plain pane you watch): with exactly one, it attaches; with more, it shows a
numbered list with one row per pane, so two agents in one session are two rows. Outside an
interactive terminal it prints the list and exits instead. To supply a credential yourself instead of pairing, give it explicitly
and replace every placeholder:

```sh
gtmux attach <host> --token <token> %7
```

<a id="what-you-get"></a>

## Using the remote session

Your local terminal becomes the Mac's tmux session: the Mac's tmux and its config,
full-screen programs, colour, all of it. It attaches a tmux client to the whole session
that pane belongs to, which is full owner access, so only you and your paired devices can
use it.

A share link is for a browser or the phone, never for `gtmux attach`: gtmux refuses a share
link, or a share code given with `--code`, before redeeming it, and the Mac's `gtmux serve`
refuses terminal access to guests. A share link grants panes, not a session; to give
someone one pane, see
[Open one pane to a collaborator](phone-and-web.md#open-one-pane-to-a-collaborator).

Two options worth knowing: `--read-only` to watch without sending input, and `--predict`
(experimental) to show your own typing immediately on a slow link.

<a id="leave-cleanly"></a>

## Disconnect

Detach with tmux's `prefix d` (by default `Ctrl-b d`) or with `Ctrl-]`. The session keeps
running on the Mac, untouched. **Do not type `exit` in the pane**: that closes the shell
in it.

---

Related guide: [Manage from your phone and the web](phone-and-web.md) · All options: [`gtmux attach`](../cli.md#gtmux-attach-work-in-a-remote-session-from-another-machines-terminal)
