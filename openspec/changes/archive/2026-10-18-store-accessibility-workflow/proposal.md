# Keep App Store accessibility metadata aligned with each submission

## Why

The preparation gate verifies package/build, text and images, but can report ready
without reviewing accessibility declarations. Labels apply to the live app and publish
independently of a candidate build, so copying the previous answers or publishing
future support early can mislead customers.

## What Changes

- Generate a private, unapproved review template bound to the exact candidate package
  receipt and a fresh live-version/declaration baseline.
- Require separate iPhone/iPad common-task evidence, capability decisions, limitations,
  change review and optional-URL review in the preparation readiness gate.
- Detect missing/stale reviews and changed live baselines. Keep future additions as
  explicit post-live work; block a candidate that regresses an existing claim.
- Provide a read-only post-live comparison, including the exact live build. No command
  publishes declarations or submits the app. Document manual ASC updates and readback.
- Put these checkpoints in the canonical workflow and submission entry points, with
  synthetic CI tests requiring neither signing credentials nor native builds.

## Surfaces

- phone and iPad: independently assessed Store declarations; app UI is unchanged.
- menubar: no runtime change; maintainer Store workflow gains a required review.
- terminal: Store helper output only; the gtmux CLI is unchanged.
- Web: no runtime change; optional Accessibility URL remains a reviewed manual field.

## Validation

Exercise package/baseline drift, missing evidence, separate device families, additions,
regressions, pagination/API failure, post-live mismatch and preparation-state invalidation.
Actual accessibility acceptance and ASC publication remain separate operator actions.
