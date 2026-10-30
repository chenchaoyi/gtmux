# Compact browser settings and conversation density

## Why
The supplied screenshot f3f2c319 shows three appearance choices hidden in a select and too much vertical space. Frequent choices should be visible and immediately selectable; settings should retain readable labels and touch-sized controls with less duplication.

The second screenshot ea080910 shows excessive spacing in expanded call lists and between conversation blocks. Remove stacked outer row padding while preserving a 44px disclosure target and clear separation between turns.

## What Changes
- Replace the three-option appearance select with directly visible native radio segments.
- Merge the single-item Display heading into Appearance; place size sliders inline and shorten the send-key hint.
- Keep typography scope, persistence, system appearance following, reset and focus/keyboard behavior.
- Compact call rows and repeated message/footer spacing without shrinking type, icons or touch targets.

## Surfaces
- terminal / attach: browser terminal preferences retained, no protocol changes.
- menubar: no native changes; this is the browser settings panel.
- phone: compact mobile web controls retain 44px hit targets and viewport bounds.
- iPad: mobile web uses the same direct controls without reducing touch targets.
- web: direct System/Light/Dark choices and compact settings layout.
