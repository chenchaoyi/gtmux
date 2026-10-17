# Stable HQ navigation and screenshot delivery feedback

## Why

User screenshots show HQ refresh/Terminal controls jumping to the trailing edge when the sidebar is hidden, an extra refresh mark crowding the toolbar, and matter rows aligned like peer section headings. Screenshot delivery only disables its target picker: editing, canvas shortcuts and title-bar Copy/Save can still alter the in-flight editor or overwrite its sending state. The menu gear contains two unwanted low-frequency entries, and the new-session prompt hides its concrete default name.

## What Changes

- Keep HQ navigation controls in a stable leading toolbar group and replace the refresh symbol in its reserved slot during refresh.
- Indent section children consistently, retaining full-row disclosure and independent selection.
- Cover the screenshot editor with a centered, accessible branded delivery state and a lightly blurred backdrop. Guard native/AppKit edits, undo and export actions during delivery; release the lock on failure, preserve the draft and show success until the existing close.
- Remove What's New and Environment check from the footer gear menu.
- Show a concrete editable suggested session name and pass that exact name on blank confirmation, without creating a session while the prompt opens.

## Surfaces

- macOS menubar app: this change owns the HQ workspace, screenshot editor, gear menu and new-session prompt.
- terminal CLI: existing new/send contracts and delivery guards remain authoritative; no CLI arguments or automatic tmux naming behavior change.
- iPhone (phone) and iPad apps: no source or protocol changes.
- Web: no source or protocol changes.

## Validation

Native window/layout checks cover leading controls through sidebar hide/show and refresh, child-row indentation, complete screenshot input/export/undo locking and failure recovery, and the displayed default name matching the creation argument. Synthetic fixtures cannot send to real panes. Check bilingual copy, reduced motion/transparency, compact/wide windows and current repository gates.
