# Codex chat retention independent of tool log volume

## Why
A long Codex rollout can put more than 64 MiB of tool output and compaction
records between user prompts. The raw tail reader then returns a single
promptless fragment. Collapse all correctly folds that fragment but appears to
have removed the conversation history.

## What changes
Stream complete Codex JSONL records from the beginning on cold load, skipping
non-chat records without allocating their full payload. Retain at most 800
parsed turns. On growth, consume only appended complete records and extend a
copy of the last turn. Keep existing response limits and collapse semantics.

## Five-surface impact
- iPhone and iPad: Chat receives earlier real turns; no native UI change.
- Web: same shared transcript reader and existing turn folding.
- macOS menu bar: no layout or interaction change.
- CLI: shared transcript consumers benefit; terminal capture is unchanged.

## Boundaries
No changes to agent logs, session ownership, terminal buffers or App Store
binaries. Existing 300-turn / 512 KiB HTTP limits remain; this is not unlimited
history paging. Other agents retain their current reader. First cold load reads
the rollout once; subsequent polling must not repeatedly scan a long active turn.
