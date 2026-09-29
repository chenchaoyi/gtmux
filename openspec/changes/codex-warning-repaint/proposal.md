# Keep Codex warning repaints idle

## Why

A Codex pane that had never run a turn received an updated weekly usage warning.
Frame sampling briefly called the redraw working; when it settled, the hub pushed
a false completion. The tmux location still had the preceding Claude session's
resume record, so the existing Codex rollout completion guard could not help.
Current Codex rollouts also name their session with `session_meta.id`, while the
cwd lookup still read only the legacy `session_id` field.

## What changes

An idle, ready Codex composer with no active or waiting marker remains idle on
screen-only repaint. Visible work and real hook turns still report working.
Radar and digest ignore a resume record that belongs to another foreground
agent. The Codex cwd lookup recognizes both session metadata field names.

## Surfaces

- The terminal, menubar, phone, iPad and web read the same radar state.
- The phone's completion push uses the corrected working-to-idle transition.
- Codex chat and digest no longer borrow the previous agent's transcript.
