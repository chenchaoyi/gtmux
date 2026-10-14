# HQ workspace

## Why
HQ's terminal mixes user instructions, supervision signals, tool output and reports.
The menu-bar card opens that terminal, while the existing readers contain no composer.
People need a durable place to follow a goal, inspect its participating conversations,
respond to HQ and verify its actions without losing context on a supervisor rotation.

## What Changes
- Add a core-owned, durable workspace: matters, explicit conversation associations,
  scoped user messages, structured HQ updates, operation history and delivery receipts.
- A matter is a goal, not a pane. Multiple conversations can participate; reference
  conversations have a bounded historical interval. Never group by title or directory.
- Add menu-bar sidebar/detail UI with New task, global HQ conversation and HQ activity.
  Render scoped conversation with the app's left/right semantics and person-battery avatar.
- Reuse dispatch verification/draft protection, transcript parsers and HQ session chain,
  event journal, agent identity/icons, Markdown renderer and board/knowledge/usage readers.
- Carry an explicit matter identifier through spawn so sibling workers inherit membership.
  Allow user-managed association/move/unlink and matter merge/split without pane operations.
- Keep delivery separate from execution. An idle agent or completed turn cannot close a matter.

## Surfaces
- terminal: additive `hq workspace` JSON/operation protocol and spawn matter association.
- menubar: first delivered workspace, local CLI consumer; current card opens it, terminal
  remains an explicit secondary action; existing readers remain available.
- phone: current HQ UI unchanged in this batch; shared protocol prepared for later adoption.
- iPad: current HQ UI unchanged; selected design supports sidebar at sufficient width later.
- Web: current HQ UI unchanged; owner-only API integration is a later batch, never guest access.

## Boundaries
No release, installation, actual HQ rotation, real-client restart or migration of personal
records during development. Existing board/knowledge/transcripts are not rewritten.
UI reads never acknowledge HQ's consumption watermark. Unknown source or status stays
explicitly unknown. Previously recorded tmux conversations can be explicitly associated as historical references; there
is no implicit title-based classification or bulk import of personal knowledge.
