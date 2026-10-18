# Tasks

- [x] Inspect installed DSH and real session record schemas without retaining private content.
- [x] Implement the bounded read-only DSH provider with sanitized fixtures and failure/identity tests.
- [x] Integrate registry, native discovery and existing scoped conversation permissions.
- [x] Wire public transcript and lifecycle state into CLI and server APIs.
- [x] Complete native Mac, phone, iPad and Web readers with official artwork and fallbacks.
- [x] Align capability specs, HTTP contract and bilingual user/design documentation.
- [x] Run the relevant gates and read-only acceptance against the running local DSH data.

## Acceptance

Reading and discovery must not mutate DSH files, start a model turn, acquire a writer, change permissions or expose private reasoning/system context. Record synthetic versus live-data verification, platform acceptance and any unsupported persisted generation explicitly.


Author verification: race-tested provider/policy/parser/hook/miner regressions; native plugin contract execution, isolated installed-DSH overlay composition, and actual installed Cordis runtime delivery to scoped temporary gtmux state; CLI read-only acceptance found one root conversation and returned public history with six source artifacts unchanged. A deliberate reasoning-filter mutation failed the public-history test. SwiftPM ran 495 tests; mobile type/lint and unit suites passed after sandbox-port and registry-fixture repairs; Web logic and Chromium scenarios cover desktop/narrow views. Local browser evidence uses synthetic data. No independent feature reviewer was assigned. Real iPhone/iPad, Safari, live native-plugin turn/OS push delivery and TUI control were not accepted. Human approval detection, usage figures and older/unknown persistence generations remain outside the declared support.
