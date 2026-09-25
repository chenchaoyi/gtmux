# The phone's chat says what is still running, and lets you get to it

## Why

When HQ dispatches work, the phone's chat is where you hear about it: "I sent that to a
worker". Then the conversation moves on, and nothing on that screen ever says what became
of it. The dispatch ledger knows — `gtmux tasks` has read it from the terminal since
hq-dispatch — but serve does not expose it, so the phone has never had the data at all.

The gap has a shape worth naming: a dispatched task that ends up WAITING on you is
invisible on the one surface you are actually holding. The radar shows the pane in red, but
you have to leave the conversation and go look for it, and nothing in the conversation
suggests there is anything to look for.

## What changes

**serve answers what is running.** `GET /api/tasks` joins two things gtmux already has and
has never put together: the dispatch ledger (what was sent, to whom, when) and the radar's
live pane status (whether that pane is waiting, working or idle). A task whose pane is gone
reports `gone` rather than disappearing, because it happened. Owner only: a guest was
invited to watch a pane, not to see everything this machine was told to do.

**The chat carries one row, above the composer.** It appears only when something is
running, says how many, and says separately when one of them is WAITING on you, because
that is the one that needs a person. It is a control with its own surface and a chevron,
in the language the approval card above the composer already uses, so that it reads as
something you can open rather than as a status line.

**Opening it is a sheet with two groups.** Running and finished, the finished one counted
and collapsible. Each row carries the goal, which agent has it, which pane it is in, and
how long it has been there. Tapping a row goes to that pane, which is what you actually
want next; a task whose pane is gone carries no chevron and is dimmed, because there is
nowhere to go.

**Only in HQ's chat.** HQ is the one that dispatches; "what is running in the background"
is its context. A worker pane's own chat does not show it: that pane IS one of the running
things, and listing it inside its own conversation says nothing.

## Not in this change

- **A stop button.** The reference this borrows from can stop a shell command with one tap.
  Stopping an agent's work here is `reap`, or a sentence sent into the pane, and neither is
  a button on a list row. Going to the pane is the action.
- **Guest access.** A share link is scoped to panes. The dispatch ledger is not.
- **The menu bar and the web.** The same data would serve both, but the menu bar already
  has the HQ card and the web page is pane-scoped; neither is where this gap hurts.

## Surfaces

- **终端 / terminal** — unchanged. `gtmux tasks` already reads this ledger; this only puts
  the same facts behind serve.
- **菜单栏 / menubar** — unchanged in this change. Its HQ card is the natural home for the
  same tally later, once the endpoint exists.
- **手机 / phone** — where this lands: the row above the composer and the sheet behind it,
  in HQ's chat only.
- **iPad** — the same component, through the shared shell. Nothing size-class specific.
- **Web** — not applicable. The shared page is scoped to panes and never sees the ledger.
