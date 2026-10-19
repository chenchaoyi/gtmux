# Fold neutral agents into the existing idle presentation

## Why

A DeepSeek pane without authoritative lifecycle evidence correctly retains neutral `running` status, but surfaces expose a separate Running / 待命 section. That asks users to learn another quiet state and disagrees with summaries that already count it as idle.

## What changes

Keep core/API evidence and completion timestamps unchanged. Group neutral agents into Idle / 空闲 alongside confirmed idle turns on menu bar, phone, iPad and Web. Keep the neutral dot, avoiding a false completed checkmark or timestamp. Preserve the separate error modifier and external/watched conversation categories. Use the existing idle label for neutral rows' accessible status labels too.

## Surfaces

- terminal / attach: preserve the core status and truthful completion evidence; no input/control change.
- menubar: one idle section and matching summary; no separate standby label.
- phone: one idle section and matching accessible labels/counts.
- iPad: use the same mobile grouping and labels.
- Web: one idle group in radar and tree, matching labels; preserve source status on each row.
