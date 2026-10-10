# Move terminal display controls out of the reading area

## Why
The bottom-left Wrap / Original control covers output, occupies two large targets, and uses a standalone colour treatment.
The Mac pane can be wider than a phone, so preserving its rows is useful but should be an optional display setting.

## What changes
Use the existing anchored menu from a neutral toolbar icon. Label the layouts Fit screen / Preserve layout, mark the current
choice, and place text-size adjustments alongside them. Remove the overlay and its 72pt tail reservation. Keep the renderer,
source-width calculation, stable scroll nesting and iOS history anchor. Disable layout/font adjustments during native selection.
Full-screen retains the chosen mode; exit it to adjust. Document wide panes, wide history and Codex's separate full-prompt reader.

## Surfaces
- Terminal (including attach): not applicable; Mac pane width and rendering are unchanged.
- menubar: not applicable; no mobile terminal renderer.
- phone: display menu replaces the bottom-left controls in Detail.
- iPad: the same Detail and menu implementation; no duplicate controls.
- Web: no change; its terminal renderer and existing display controls are separate.
