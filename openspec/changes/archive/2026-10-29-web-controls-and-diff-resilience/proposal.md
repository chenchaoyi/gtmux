# Simplify browser controls and bound native Diff rendering

## Why
Session modes shift in focus, tools require an unnecessary menu, font controls repeat and automatic panel opening adds complexity. Native Diff eagerly mounts every line and treats failures as empty content.

## What Changes
- Direct, spaced Trace/Diff actions beside stable Chat/Terminal controls.
- Settings owns font size; remove automatic panel opening; use the canonical HQ mark.
- Virtualize native Diff, bound long-line measurement and expose errors/retry.
- Remove the competitor comparison from both README sources.

## Surfaces
- terminal: no command or wire changes.
- menubar: existing behavior retained.
- phone: bounded Diff, error/retry; mobile browser toolbar.
- iPad: same bounded Diff; browser toolbar adapts by width.
- browser: stable modes, direct tools, HQ identity and fewer controls.
