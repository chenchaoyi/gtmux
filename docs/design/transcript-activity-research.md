# Transcript activity presentation

Paired Chinese document: [中文](transcript-activity-research.zh.md).

The reported phone screen displayed a run of bare `js` and `wait` lines. The problem is hierarchy and missing descriptions, not the number of agent statuses.

## First-party references (checked 2026-10-19)

- [Claude Code Desktop, view modes](https://code.claude.com/docs/en/desktop#switch-view-modes): Normal folds tool calls into summaries while retaining full replies; Verbose exposes individual calls. Borrow progressive disclosure, without introducing another global mode selector.
- [Codex activity groups](https://github.com/openai/codex/blob/e4568a3538c9cab6627d64ff8cdfc7d98b79b7db/codex-rs/tui/src/history_cell/activity_group.rs): adjacent calls keep their order and retain detail positions as activity changes. Borrow an ordered activity group between reply segments.
- [Codex terminal interaction history](https://github.com/openai/codex/blob/e4568a3538c9cab6627d64ff8cdfc7d98b79b7db/codex-rs/tui/src/history_cell/exec.rs): polling/input bookkeeping is kept in detailed/raw history rather than repeated normal-chat notices. Borrow a compact summary instead of a wall of opaque tool names.

The Codex references are its official open-source terminal interface, not an inspected private desktop implementation. The Claude reference is official documentation; neither desktop app was operated for this comparison. No source code or visual assets were copied.

## Decision

Each segment's calls share a quiet bordered Activity card, distinct from a reply bubble. Its heading names the process, counts calls, and summarizes tool frequencies (first three distinct names, then a count of additional distinct tools). Expand to reveal numbered rows with the recorded tool name above the argument summary. Neutral text and separators carry hierarchy; no cyan tool-name wall, success checkmarks or extra agent state. Phone/iPad keep the existing live-open/history-folded policy and reader override; focused Web keeps its history disclosure policy. Existing compact workbench tiles still show reply text only.

Codex's explicit string `title` then `description` is used before existing command/path fallbacks. The parser still collapses whitespace and limits summaries to 80 Unicode code points plus an ellipsis. Surfaces render literal selectable text; descriptions are never interpreted as HTML or Markdown. Unknown tools and older cores retain their actual names.

## Boundaries

A logged call does not prove it succeeded or finished. The current step contract has no result, exit code or duration; the card claims none of these. It does not expose full arguments, stdout/stderr or full command bodies. Missing descriptions cannot be reconstructed from `js` alone. Completed transcript data already cached by a running older server uses that server's summaries until the normal update/restart; this change does not clear logs or running sessions. Rendering checks use synthetic fixtures, not real agent execution.
