# Reconcile resumed Codex rollouts by conversation ID

## Why

ChatGPT desktop can continue one Codex conversation in a second rollout whose
filename ends in `<session-id>_<instance-id>.jsonl`. gtmux looked only for the
unsuffixed file. A completed conversation could therefore stay `working`, show
an old activity time, and lose later turns in Chat.

## What changes

Find all rollout files for the conversation, confirm suffixed files against
`session_meta.id`, and use the latest turn boundary across them. A later
`task_complete` or `turn_aborted` ends a native working turn; a later
`task_started` keeps it working. Read conversation times and Chat turns across
matching files in rollout order, retaining the existing bounded per-file cache.
The Chat ETag tracks all matching files; raw byte-offset readers keep their
original file identity.

## Surfaces

- **terminal (CLI and attach)**: the radar reports the corrected native status
  and activity time; attach itself remains a tmux connection.
- **menubar**: the existing native row consumes the corrected radar status.
- **phone and iPad**: the shared native row and Chat view receive the corrected
  status and combined turns from the Go server, without a mobile client change.
- **Web**: the existing radar and Chat consumers receive the same server data.

## Non-goals

No inference from another conversation's file, from file modification time, or
from ChatGPT desktop UI state. Desktop sessions remain read-only in gtmux.
