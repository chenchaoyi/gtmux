# Portable iOS test publication setup

## Why

A second Mac reached publication without the first machine's private configuration.
The runbook recorded a local file location but had no initialization path or early
check. A Git checkout must not imply that credentials, signing or connectivity move
with it.

## Hosted delivery design

Use a manual, opt-in GitHub Actions workflow as the preferred build/publication
path after operator setup. Validate a published release on main with green CI
before entering a protected signing environment. A separate UI-preview mode accepts
an exact CI-green main commit without creating a release/tag/store submission.
Preview packages carry a distinct build number, commit label and test-only notes. Import three Ad Hoc profiles and
an iOS distribution identity into an ephemeral runner. Reuse the private ccy.dev
uploader with a read-only repository credential; do not copy its private source
into this public repository. Publisher secrets are step-scoped. Encrypt installer
URLs, IPA and diagnostic output to an operator-held age identity before uploading
any recovery artifact. Only public progress/status reaches logs and summaries.
Fork/PR checks never receive signing/publisher secrets or publish packages.

Environment setup, credential migration and a real signed publication require
operator review; this patch does not configure or trigger them. Local delivery
remains the documented fallback while hosted delivery is not enabled.

## What Changes

- Add a maintainer-only adapter for explicit/XDG/default configuration paths and CI
  secrets, with local preflight and non-overwriting stdin import.
- Delegate publication/link issuance to the existing ccy.dev uploader; keep both
  output streams private and propagate failures.
- Document pre-build checks, verified secure transfer, local uploader/runtime paths
  and the boundary between readiness and real installation in English and Chinese.
- Add synthetic tests to local and CI gates. Do not transfer real secrets, create
  credentials, open SSH access, deploy a service or release/install an App.

## Impact

Maintainer release workflow only; no runtime App, CLI/API, five-client UI or signing
behavior changes to production Apps. Hosted signing is new maintainer infrastructure. No confirmed SSH route to the original release Mac exists. Real
cross-machine credential transfer remains operator setup, not a passed acceptance.

## Surfaces

- terminal: maintainer script/preflight only; no gtmux CLI behavior change.
- menubar: no product changes.
- phone: no App changes; only test delivery setup changes for release operators.
- iPad: the same test delivery workflow; no UI or device acceptance claim.
- Web: no website/Worker changes or deployment; existing publication service reused.
