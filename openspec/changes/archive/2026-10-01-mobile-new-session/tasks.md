- [x] Implement shared core creation and owner-only HTTP endpoint with receipts and diagnostics.
- [x] Add paired-app creation entry points, bounded form and terminal navigation.
- [x] Cover permission, retry, duplicate-name, error and navigation regressions.
- [x] Verify UI and gates; sync bilingual docs/API/spec/notes and archive.
- [x] Evaluate high-value follow-up capabilities across the whole product from repository evidence.

## Verification
- `GOFLAGS=-p=2 make check`: format, vet, staticcheck and race tests passed.
- `npm run check -- --runInBand`: TypeScript, ESLint and 109 suites / 1199 tests passed (6 pre-existing skips).
- `CGO_ENABLED=0 GOFLAGS=-p=2 go build`: passed.
- `tsc --project mobileapp/e2e/tsconfig.json --noEmit`: passed after separating Node test configuration from React Native bundler settings and materializing async WebDriver arrays in existing tests.
- New end-to-end test body executed with its actual fake server and assertions in an isolated Appium harness; passed. The broad e2e global setup was not run because it can reclaim other sessions' drivers.
- Cached-debug iPhone 17 Pro: English/dark and Chinese/dark forms with visible focused keyboard, duplicate name and single-tap create/open checked.
- Cached-debug iPad Air 11-inch: Chinese/light centred form with keyboard, duplicate name and new pane in the main canvas checked.
- `bash scripts/check-design.sh`: passed; main specs synchronized.

## Limits
No native app rebuild, release, real-Mac tmux mutation, real-device installation or physical accessibility/layout acceptance. Request receipts are live-session scoped and serialize one serve process, not independent concurrent serve processes. The capability review records proposed follow-up work, not shipped functionality.
