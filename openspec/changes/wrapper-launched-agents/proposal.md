# wrapper-launched-agents — bring back the agent the way it was started

## Why

A Codex session did not come back after a reboot. Chasing it turned up two separate
things, and only the second is a defect.

**What happened this time was correct.** The saved layout recorded that pane as a bare
shell (`bash`, with no full command), because the agent had already exited before the
save was taken. Restore brought back a shell, which is what the save said was there —
and the guard that refuses to inject a resume into a pane that never ran an agent is
there on purpose (it was added after restore did exactly that, #688).

**What it exposed is real.** That session was not started with `codex`. It was started
with `opencrab`, an internal wrapper. gtmux's resume record holds
`{agent: codex, sessionId, cwd}` and nothing about how the agent was launched, and the
agent registry answers "how do I resume codex" with a fixed `codex resume <id>`. So even
if the process HAD been alive at save time, the conversation would have come back —
if at all — as a different command than the one that created it: no wrapper, and
whatever configuration, credentials or environment that wrapper exists to provide.

Claude sessions survive a reboot for a reason that hides this: tmux-resurrect saves the
whole command line (`claude --resume 9a65ce9e…`) and restores it verbatim. gtmux's own
path is the complement, and it is the one that assumes the launcher is the agent's own
name.

## What changes

Record HOW the agent was launched, alongside what it is and where. The hook already
walks the pane's process tree to identify the agent (that is how a Claude pane whose
`pane_current_command` is a version string gets recognised at all); the launch command is
visible at the same moment. A resume then reproduces the command that created the
session, and falls back to the registry's generic form only when nothing was recorded.

This covers wrappers, aliases, `npx`-style launchers, and anything else a user's real
environment puts in front of an agent — none of which the registry can enumerate.

## Boundaries

- **restore is the most delicate path in this repo.** It brings back a working set after
  a reboot and has cost real work before: phantom `claude --resume` injection into panes
  that never ran an agent, an archive with shifted fields, a `last` pointing at an empty
  save. This changes WHAT is typed to bring a conversation back, so it must not change
  which panes are considered resumable at all.
- **A recorded command is not a licence to run anything.** It is replayed only for a pane
  the existing evidence rules already agree ran an agent, and only to resume the
  conversation the record names.
- The generic registry form stays the fallback: a record written before this exists, or
  by a path that could not see the launch, must still resume as well as it does today.
- This does not make an agent that exited before the save come back. Nothing can: the
  save is the only account of what was running.
