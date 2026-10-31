- [x] Implement shared native code rendering and reading controls.
- [x] Cover exact source, language detection, token budgets and removed drag controls.
- [x] Exercise phone/iPad native gestures, light/dark and large-text states with an independent synthetic fixture.
- [x] Update paired design documentation and the mobile capability spec.

## Acceptance
- Latest mobile gate: 2071 tests passed; 15 pre-existing skips.
- Native XCUITest: phone 7/7, iPad 7/7 and accessibility-size phone 3/3. The same content drag reproduces page-back with iOS 26 defaults, stays on the code screen with the fix, and edge-back still works. Reader open/vertical scroll/close and native selection were exercised.
- Native fixture uses current JavaScript with a compatible previously built native host; package locks match and the generated ReactCodegen checksum difference is recorded. It does not replace a production unsigned build or signed installation.
- Physical-device interaction and actual VoiceOver speech were not exercised. Accessibility labels and target geometry were inspected separately.
