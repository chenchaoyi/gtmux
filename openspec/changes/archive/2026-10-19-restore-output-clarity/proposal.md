# Make restore output scannable

## Why

The normal restore output mixes a workspace inventory, long historical prompts and repeated layout diagnoses. In a large workspace, the warning and actual outcome become hard to find.

## What changes

Show a width-aware compact session table in normal restore output. Keep bounded conversation excerpts in the existing read-only `--plan` preview and preserve its JSON contract. Summarize layout differences with short per-window facts; retain full evidence in the restore log. Separate progress and outcomes, and avoid claiming the layout or screen contents were fully recovered before validation.

## Scope

CLI presentation and documentation only. Restore matching, timing, commands, permissions and save handling remain unchanged. No live restore or agent input is required for validation.
