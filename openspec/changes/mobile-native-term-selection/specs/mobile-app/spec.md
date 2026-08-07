# mobile-app (delta)

## ADDED Requirements

### Requirement: Native terminal text selection on iOS

The iOS terminal viewer SHALL provide system-native text selection over the
colored terminal rendering: long-press selects the word under the finger with
the standard selection band and BIDIRECTIONAL drag handles, and Copy places
exactly the selected range on the clipboard. Selection geometry SHALL be
supplied by the app (uniform row grid + measured character advances), never
inferred by a second text-layout pass, so the band always lands on the glyphs
the user sees regardless of CJK content or line wrapping.

#### Scenario: Select a range anywhere in the buffer

- **WHEN** the user long-presses a line — at the live tail or deep in
  scrollback — and drags either handle in either direction
- **THEN** the selection band tracks the touched glyphs exactly, the terminal
  does not jump or auto-scroll, and Copy yields precisely the banded text.

#### Scenario: Zero standing cost

- **WHEN** no selection is active
- **THEN** the selection layer performs no text layout and typing/scrolling
  performance is unaffected by its presence.
