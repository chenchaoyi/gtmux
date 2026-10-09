# Preserve conversation ownership during restore

## Why
A restored `check-hq-stop` pane had no exact record. On 2026-10-08 the cwd/position fallback found two candidates, selected the newest and resumed the `gtmux dev` conversation, whose original pane was already live. Both Codex clients then displayed the same conversation. Directory, position and recency do not prove a rename.

## What changes
- Reserve conversation IDs already bound to non-shell live panes before any resume.
- Exclude fallback records whose original locator still exists in the saved layout or live topology; these are not rename evidence.
- Accept only one distinct remaining conversation; refuse ambiguous matches and report the skip to the user and diagnostics.
- Apply identical fallback ownership rules in the read-only restore plan.
- Preserve exact/saved-command recovery and uniquely evidenced renames.

## Surfaces
- Terminal: safer restore selection with explicit skipped-candidate receipts.
- Menu bar: uses the same restore command and plan; no new controls.
- Phone: radar no longer receives conversations duplicated by this restore defect; no UI change.
- iPad: same core behavior; no UI change.
- Web: same core behavior; no UI change.

## Verification
Synthetic selection and plan tests plus an isolated real tmux test reproduce the live-owner case without reading, typing into or terminating operator sessions. Run make check and design gates. Existing duplicate live clients are not automatically stopped or reset.
