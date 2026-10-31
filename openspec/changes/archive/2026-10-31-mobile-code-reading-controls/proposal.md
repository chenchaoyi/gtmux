# Compact mobile code reading controls

## Why
Two help paragraphs below tool input compete with the actual payload and consume chat space.

## What changes
Use a draggable horizontal scrollbar in the existing input header, synchronized with actual code scrolling and accessible adjustment actions. Use an information icon for partial/summary-only input, with the explanation on demand. Keep exact text/copy, vertical limits and reader state. Keep Diff vertical indicators at the visible viewport edge while horizontal content is wider, with a light indicator in a fixed gutter on the dark reading canvas, driven by actual content/viewport/scroll readings without re-rendering the row list for vertical movement. No dependency or permission changes.

## Surfaces
- terminal: no change to input or PTY.
- menubar: no change.
- phone: compact header controls in shared ToolSteps.
- iPad: the same responsive controls.
- Web: no change to browser tool previews.
