# Conservative PR checks by change scope

## Why

All six current CI jobs run even for prose-only changes, while no PR job renders
the Web UI or compiles the iOS app/extensions. Scanner startup failures are also
swallowed and can report a clean vulnerability scan. Outside contributors need
automatic, understandable feedback without deployment secrets or manual release approval.

## What Changes

- Always run policy, documentation examples, strict specs and CI regression checks.
- Select complete module suites using an explicit path policy; unknown/shared
  impact, dependency/build/CI changes and unreadable diffs run all suites.
- Add a stable aggregate that fails on required-job failure, skip or cancellation.
- Add rendered browser smoke tests and unsigned iOS simulator compilation only
  when affected; preserve existing Go, Swift, RN and Worker suites.
- Fail incomplete vulnerability scans; retain the existing advisory/fix policy.
- Explain the plan in GitHub's summary and document its limits in both languages.

## Surfaces

- terminal CLI: no runtime behavior change; core tests and cgo-free builds remain gates.
- menubar: no runtime UI change; Swift build/tests run for menu-bar or uncertain changes.
- phone: no runtime UI change; RN tests and unsigned native compilation become scoped gates.
- iPad: same RN/native checks as Phone; real layout/VoiceOver remains separate acceptance.
- Web: no runtime UI change; actual embedded DOM/CSS is tested at desktop/narrow widths.

## Non-goals

No release, signing, installation, Worker deployment, credentials, branch policy
change or live-HQ/terminal operations. Fixture browsers do not prove real-server
contracts, Safari, VoiceOver, APNs or visual polish.
