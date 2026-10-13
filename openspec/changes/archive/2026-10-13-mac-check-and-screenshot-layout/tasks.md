## Implementation
- [x] Remove duplicate check results while retaining full guidance and legacy reports.
- [x] Support multiline notes and safe explicit send key handling.
- [x] Fit image/marks to live window canvas; bound initial screen placement.
- [x] Verify keys, resize geometry, export resolution and detail content.
- [x] Update paired design/user docs and run required gates.

## Acceptance
Native isolated tests cover actual note input, IME protection, binding, live host
resizing, annotation pointer coordinates and pixel-preserving exports. Installed-app
screen-capture interaction and VoiceOver are not verified; the existing native UI
automation service timed out in prior acceptance. No release or installation was
performed for this change.
