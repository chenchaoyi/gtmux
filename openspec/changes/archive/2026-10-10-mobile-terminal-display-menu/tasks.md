## Implementation
- [x] Move display and terminal text-size choices into the shared anchored menu.
- [x] Keep selected state, selection protection and stable scroll nesting.
- [x] Update English and Chinese design/user docs, including the stale Codex description.
- [x] Verify component/Detail integration, mobile checks and repository gates.
- [x] Sync spec and archive.

## Acceptance
Physical iPhone/iPad layout and spoken VoiceOver are not yet verified. No native build or installation is part of this change.

## Verification
- Mobile check: TypeScript and ESLint passed (existing warning baseline), 153 suites / 1,628 tests passed; 15 tests skipped.
- Renderer tests cover preserved rows, source-width fallback, selection guards, scroll identity and an actual history offset restore.
- Detail/menu tests cover selected accessibility state, disabled actions, font bounds, full-screen and chat persistence.
- `make check` passed. No native binary, release or installation was produced.
