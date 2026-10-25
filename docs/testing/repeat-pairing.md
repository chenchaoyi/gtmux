# Repeated pairing audit (2026-10-25)

Repeated valid owner scans refresh one saved endpoint without losing its settings.
Concurrent callbacks/taps redeem once; failed credential writes preserve the usable
connection and offer Retry saving. Canceled operations and old credentials cannot
select or overwrite a replacement connection. Baseline: `d5029932`; branch:
`fix/repeat-pairing`. Nothing was released, installed or versioned.

The detailed [Chinese scenario matrix](repeat-pairing.zh.md) records each cause,
regression and validation boundary. iPhone and iPad share the React Native screens,
not SwiftUI. The Go owner enrollment code remains single-use, five minutes, and
invalid after serve restarts. Each fresh code creates a new device credential;
previous credentials and device-roster rows are not automatically revoked or merged.

There is no stable global server UUID. Identity is the canonical endpoint plus
previously authenticated `/api/addresses` alternatives. Trailing slashes, host case
and explicit standard ports collapse; names, Direct location IDs, unknown host/IP
aliases, schemes, non-default ports and routing paths are not interchangeable.
Unknown aliases may still produce separate entries. Existing tokens are never sent
to an unfamiliar scanned endpoint to guess identity. Owner alternatives do not merge
a guest connection.

Scope snapshots are also tied to their client: a new guest credential never inherits
the old owner controls when its scope query fails. That additional regression failed
on the previous implementation.

The first baseline regression run failed seven assertions (75 passed): lost address
metadata, duplicate entries, credential replacement after failed writes, lost
concurrent updates and an unbounded response body. Two independent connection tests
also failed: late old-credential failure overwrote fresh success, and late old success
overrode fresh failure. New UI regressions also fail on the original screens; cascading
harness failures are not counted as additional defects. Temporary baseline copies were
confined to this worktree and verified with `cmp` before restoring the fixed sources.

Saved-list changes now serialize against the latest committed snapshot. Duplicate
healing prefers the selected credential and settings, never arbitrary row order. One Keychain
blob holds both list and selection and is written before applying state. A redeemed
credential is retained only in the current sheet for a failed-save retry; leaving or
losing the redemption reply requires a new code. Cancel is disabled during the native
write commit. Success closes the sheet, including re-pairing the current URL. The
15-second enrollment deadline covers headers and body. Old HTTP/SSE callbacks are
scoped to the originating client and mount; the previous subscription closes.

Structural camera tests cover 320×568, 568×320, 393×852, 852×393, 768×1024 and
1024×768. The window shrinks on short landscape phones, keeping title and Cancel.
Camera callbacks and English/Chinese feedback are tested through component injection.

Reproduce the gates:

```sh
cd mobileapp
npm run check -- --runInBand
cd ..
go test -race ./internal/server -run 'TestRepeatPairing|TestEnroll' -count=1
GOFLAGS=-p=2 make check
./scripts/check-design.sh --source-only
```

The first full mobile gate passed 188 suites / 1911 tests with 15 externally gated
skips; additional regressions were added before the final run. The first core run
passed format/vet/staticcheck but three existing internal/app strict timing tests
failed (brew 5s, native hook 5s, version probe 2s); the complete gate is repeated with
limited package concurrency, without changing deadlines. The complete `GOFLAGS=-p=2 make check` passed,
including fmt/vet/staticcheck, all race tests, 75 publisher tests and 44 CI/policy
tests. The final mobile gate passed 189 suites / 1928 tests, with 15 externally gated
skips, TypeScript passing and ESLint reporting no errors. The final source gate
passed.
The source gate reports its private exact-term policy unavailable; that portion
is skipped, while architecture, bilingual docs and strict specs are checked.

The HTTP tests run the production Go handler and enrollment manager over random
loopback ports with synthetic credentials. They prove concurrent single use, reply
loss refusal, fresh-code recovery, continued validity of old credentials, rejection
by another server, expiry and restart invalidation. They never use a user serve,
tunnel, device or credential. Component tests mock camera decoding, fetch and native
Keychain. These layers are not a real-device end-to-end run. Physical camera scanning,
native iPhone/iPad screenshots, native Keychain interruption/restart behavior,
system-network faults, native SSE and APNs remain unverified. No client was installed
and no other executor's simulator or checkout was operated.
