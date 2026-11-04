## Why

Release 171 spent 6m07s in CLI release work before starting a 7m54s Mac build.
iOS preparation also waited for CLI publication. Compilation, main CI, signing and
publication are distinct gates; overlapping preparation can remove idle time without
weakening any final publication gate. Savings overlap and need a later signed-release
measurement; no new release or signing invocation is part of this implementation.

## What Changes

- Build the Mac App independently; stage immutable bytes and publish only after CLI and Mac success.
- Validate cross-job source/ref/run/original-attempt/length/SHA256 and preserve failed-only reruns.
- Build iOS from a reviewed annotated tag before the GitHub Release exists; recheck publication, latest CLI/main checks and remote tag before uploader invocation.
- Bound read-only CLI waiting and retain encrypted diagnostics on failure.
- Update both maintainer languages and specs; retain protected environment, signatures, full CI and access policies.

## Surfaces

Maintainer infrastructure only. Terminal product commands, menubar, Phone, iPad,
Android and Web product UI are unchanged. No new release, Store submission,
installation, signing identity, runner registration or historical cleanup is authorized.
