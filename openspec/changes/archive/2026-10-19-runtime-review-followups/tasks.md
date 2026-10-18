## Implementation

- [x] Bound legacy transcript caches with eviction and correct cold/append re-reads.
- [x] Bind and lease serve before credentials/status, including competing-process regressions.
- [x] Align Go relay APNs errors and Live Activity/wire behavior using cross-runtime fixtures.
- [x] Bound the reviewed event/diff read paths and retain cursor/filter semantics.
- [x] Reuse the shared serve radar producer with a tested standalone CLI fallback.
- [x] Bound the owned notification backlog without touching other state.
- [x] Distinguish OpenSpec invocation/setup failures from actual invalid specs.
- [x] Record review dispositions and synchronize affected current specs and paired user docs.

## Acceptance

Regression and full gate results are recorded in the PR. Real APNs delivery, banner appearance, AppKit UI behavior and physical-device acceptance are not established by synthetic tests. No independent review of this iteration is claimed. The separate iPad installation remains dependent on its real device identity and signing coverage; mobile source is unchanged here.
