# Keep the menu visible during app-initiated work

## Why

Restore opens terminal windows, which dismiss the transient menu and hide its progress. Automatic release notes explicitly replace the menu before it can be used. 

## What Changes

Preserve the current menu presentation across these programmatic focus changes, while keeping explicit dismissal and ordinary transient behavior on subsequent openings. No CLI restore semantics or release acknowledgement policy changes.

## Surfaces

- terminal / remote attach: unchanged; Restore still uses the existing CLI.
- menubar: preserve the open list during Restore and automatic release-note presentation.
- phone: not applicable; no NSPopover.
- iPad: not applicable; no NSPopover.
- Web: not applicable; no NSPopover.
