# Tasks
- [x] Add durable verified-session policy, CLI and owner HTTP endpoint.
- [x] Apply policy to radar counts, digest/HQ debt and pull, notifications/Live Activity and mining.
- [x] Implement Mac and shared phone/iPad per-session settings with honest save feedback.
- [x] Update Web presentation, contract, bilingual documentation and specs.
- [x] Verify meaningful regression tests and repository gates.

## Acceptance
Real iPhone/iPad acceptance requires an unlocked connected device. No release or installation is part of this change.

## Verification results
- `make check`: fmt, vet, pinned staticcheck and all Go race tests passed.
- Mobile: TypeScript/ESLint passed; all 1581 active Jest tests passed (15 existing skips). The existing App smoke test now unmounts its async root, clearing the leaked server-list refresh timer; no force-exit workaround.
- Swift: all 353 tests passed; the new policy/count tests pass. Temporary native render checks confirmed bounded content/footer geometry but do not replace interactive device acceptance.
- Mac desktop-only list visibility and header counts, plus iOS native modal handoff/cancellation, are regression-tested.
- Web: all 24 Node interaction tests passed.
- Owner/guest, stale save, independent permissions, revocation, future-only mining, same-second reenrollment, identity and other-agent regression boundaries are covered.
- Real iPhone/iPad VoiceOver/layout acceptance remains pending connected unlocked devices.

- Negative controls: temporarily bypassing the digest exclusion and mining consent gates made their regression tests fail; restoring the gates passed. Removing the large-header recovery buffer also made its regression test fail; restored recovery passes.

- PR CI exposed Staticcheck v0.8.1 incompatibility with Go 1.27.2 export data. Only lint uses the verified Go 1.27.1 analysis toolchain; builds/tests/vulnerability checks retain CI current Go.
