# Structured transcript activity

## Why

Tool steps are bare `js` / `wait` lines between replies. The phone screenshot shows no hierarchy, and Codex tool descriptions already present in arguments are discarded.

## What Changes

- Render intermediate calls in a neutral disclosure card on phone, iPad and Web: activity heading, call count, compact tool-count summary, then ordered rows with the actual tool name and bounded argument summary.
- Prefer explicit Codex tool `title` / `description` metadata before command/path fallbacks; keep the current 80-rune summary bound.
- Preserve mobile live/history expansion defaults and reader choices; keep Web history collapsed. No new agent states, result ingestion, success inference or remote command execution.

## Impact

Transcript presentation only. No API shape change. Existing parsers for other harnesses continue to supply the same steps. Reference and limits: docs/design/transcript-activity-research.md.
