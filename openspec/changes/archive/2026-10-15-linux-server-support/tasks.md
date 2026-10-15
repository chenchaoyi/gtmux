## Implementation
- [x] Add platform adapters and Linux resource sampling with missing-source semantics.
- [x] Add systemd user services and port remote lifecycle, recovery and update restart.
- [x] Enable Linux installer, hooks, headless desktop behavior and appropriate doctor checks.
- [x] Update shared UI terminology, capabilities and bilingual documentation.
- [x] Add Linux runtime/install/service tests and run required gates.

## Acceptance
Local full Go/race, mobile, installer, Web logic, cross-build and mutation checks passed. Linux runtime and real systemd lifecycle are required CI checks; their results belong in the PR. Real iPhone/iPad layout, real agent accounts, push delivery and Linux logout/reboot persistence require separate acceptance; CI lifecycle tests must not be described as those checks.
