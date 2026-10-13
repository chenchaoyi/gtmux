## Implementation
- [x] Add bounded bilingual release-note JSON through the existing CLI parser.
- [x] Add upgrade detection, baseline migration, cache, acknowledgement and stale-read protection.
- [x] Add the native reader and permanent menu/preferences entries.
- [x] Verify core bounds/failures and native lifecycle/layout with regression guards.
- [x] Update bilingual docs/design and archive the implemented change.

## Acceptance
Installed-app interaction and VoiceOver have not been verified. The native UI
automation service timed out in the preceding installed-app checks. Isolated
model and native-host tests are distinct from live acceptance. No release or
installation is part of this implementation task.

Core tests cover both language blocks, stable-release filtering, installed-version
bounds, text compatibility and failed/invalid requests. Removing the JSON upper
bound makes its guard fail; acknowledging at fetch completion makes the lifecycle
guard fail. Native tests cover launch migration, fresh/same/downgrade behavior,
skipped releases, cached rereading, retry, source gaps, stale reads and long text in
420/640-point hosting views. Native hosting previews were inspected; they are not
installed-app or VoiceOver acceptance.
