# Verification

- Screenshot evidence: expanded doctor rows repeated Installed/on in their headings
  and detail bodies. The new detail model excludes duplicate labels, purposes and
  short results, but retains unique guidance and full long results.
- Actual native Return inserts newline; Command-Return sends only explicitly and
  blocks disabled sends, key repetition and marked input-method text. The native
  binding retains first\nsecond; the fixed note view tracks its viewport width.
- Native hosting window is resized in both directions. Canvas image and marks keep
  the same aspect ratio and scale; pointer coordinates remain in image space.
  Retina 600×400 export PNG is unchanged before/after resizing.
- The initial window uses the capture's point size when possible and bounds larger
  windows to the current display's usable frame, measuring actual title-bar height.
- Full Swift suite: 403 tests passed. make check: Go race tests and 32 publisher
  tests passed. Strict OpenSpec validation and design conformance passed.
- No actual agent send, TCC permission change, install or release. Physical display
  capture and installed-app/VoiceOver acceptance remain separate from these tests.
