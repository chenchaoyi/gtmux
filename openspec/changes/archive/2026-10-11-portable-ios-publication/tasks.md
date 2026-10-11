## Implementation

- [x] Add configurable local preflight and validated private stdin import.
- [x] Delegate to the existing uploader with private outputs and exact exit status.
- [x] Update bilingual runbook and repo instructions for a fresh release machine.
- [x] Cover a different HOME, XDG/explicit paths, env-only setup, unsafe files,
  failed imports, secret-output containment, retries and execution failures in CI.
- [x] Prepare opt-in hosted build/signing/publication and encrypted recovery.
- [x] Test release trust gates, profile validation, output privacy and failures.
- [x] Run local gates and review the implemented workflow against its spec.

## Acceptance

Synthetic isolated directories cover fresh-machine behavior; no real credentials
were transferred to another Mac. HQ has no confirmed source-Mac SSH route. Local
configuration checks do not validate remote token access or Apple signing. No
native build, release, installation, credentials or production configuration change.

Validation: 28 synthetic tests passed; real age encryption/decryption round-trip
passed using a temporary synthetic identity; actionlint and all 12 shell syntax
checks passed; make check and design/OpenSpec gates passed. Hosted UI-preview and
release-test are implemented but remain disabled: no protected environment, iOS
secrets, private-repository reader or publisher secrets were configured, and no
real hosted archive/export/publication has run. Only CI-green main commits are
accepted; feature/fork signing is intentionally outside this first delivery.
