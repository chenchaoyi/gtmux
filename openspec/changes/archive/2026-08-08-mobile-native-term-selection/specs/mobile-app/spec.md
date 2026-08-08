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

#### Scenario: Any Dynamic Type size

- **WHEN** the device's text size (Dynamic Type) is set to any value — below,
  at, or above the default — and the user long-presses anywhere in the buffer,
  including the bottom-most rows at the live tail
- **THEN** selection works identically across the whole buffer: the rendered
  terminal honors the OS text size, and every geometry consumer (row wrap,
  row height, the selection overlay's font) derives from the same effective
  font size, so no region of the buffer is dead to selection.

#### Scenario: Zero standing cost

- **WHEN** no selection is active
- **THEN** the selection layer performs no text layout and typing/scrolling
  performance is unaffected by its presence.
