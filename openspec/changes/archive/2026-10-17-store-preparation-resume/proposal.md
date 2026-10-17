# Resumable App Store preparation

## Why
The 1.0.121 preparation repeated native builds across five capture failures, detected a resigning permission difference after upload, and obscured Apple image processing progress. Preserve validated work and make incomplete states explicit.

## What Changes
- Separate unsigned simulator build, checkpointed capture and current-revision rendering; verify provenance before reuse.
- Validate Store package identities, permissions, original resources and archive symbols before binary upload. Retain sanitized archives/symbols in encrypted delivery recovery.
- Add bounded, visible ASC processing progress and an independent preparation status command with durable private evidence.
- Document source/harness/render revisions, recovery and user-only final submission in English/Chinese.

## Non-goals
No new release, upload to the live listing, credential creation, submission, real-device acceptance, or device-job parallelization in this implementation task.

## Surfaces
Maintainer tooling only. CLI/server, menu-bar, mobile runtime, Web and remote terminal behavior are unchanged; simulator capture exercises the existing mobile release. No new app release is required for these repository workflow changes.
