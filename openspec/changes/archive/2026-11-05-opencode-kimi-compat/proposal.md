# Update recorded tool coverage for current agent runtimes

## Why

OpenCode 2.2.5 has a different plugin contract and a default shared service. Kimi
2.5.0 records tools as loop tool.call events rather than assistant message toolCalls
in real noninteractive runs. The old bridge/parser can lose calls and cannot use
a shared service's inherited terminal pane as session ownership.

## What Changes

- One OpenCode module supports verified V1 server and V2 setup entrypoints, preserving user configuration and terminal stdin.
- Managed terminal launch/restore use a version-aware native handoff; V2 private server ancestry is verified before terminal state/content writes. Shared unconfirmed sessions stay status-only.
- Kimi reads recorded loop calls and older message calls; shared exact-ID dedup/input budgets and source-scoped full input work across providers.
- Recorded edit inputs preserve selected original/replacement fields, including empty strings; results/reasoning/unknown configuration stay excluded.
- Add observed synthetic runtime fixtures, plugin/ownership/restore regressions and phone/tablet trace coverage; update bilingual integration guidance.

## Surfaces

terminal: new gtmux opencode handoff and restore compatibility. menubar/HQ:
existing session lifecycle and receipts get correct root facts. phone/iPad/Web:
Chat and Trace consume the same recorded operations and complete selected inputs.
No account, shared-service restart, follow/input permission or access-policy changes.

## Release reliability follow-through

The prior exact-main Mac gate failed to acquire an ARM runner three times, without
running code. Validate the standard Intel Mac host with both-architecture app and
Widget builds and the unchanged native test/design/signing/notarization checks.
iOS stays on ARM. A snapshot rehearsal must verify the actual staged universal
artifact before using this host for the authorized release.

The actual rehearsal compiled and signed both architectures and received Apple's
Accepted status, then failed ticket retrieval with a CloudKit network loss. Add a
bounded retry for that exact transient lookup only, explicit Accepted JSON checking
and mandatory final ticket validation. Submission, signing, permanent failures and
publication permissions keep their existing boundaries.

Repeated latest-image ARM acquisition failures also block iOS compilation. Validate
the other documented standard ARM image, macos-15, with explicitly installed
Xcode26.3 in both unsigned CI and protected delivery. Reject a non-ARM host, compile
the same ARM64 app/extensions, and retain every signing/publication boundary.
