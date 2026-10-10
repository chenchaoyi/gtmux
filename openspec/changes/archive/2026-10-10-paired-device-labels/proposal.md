## Why

The Mac roster still prints legacy `browser` while the phone prints Browser, and old browsers have no recoverable details until reconnecting. iOS restricts user-assigned device names; generic iPhone/iPad labels cannot distinguish devices. Add an explicit editable roster label on the Mac, without claiming it is the system name or requiring new Apple entitlements.

## What changes

- Mac-only paired-device label editor; master-only API, stable ID/token and retained platform metadata.
- Browser metadata captured on enrollment and authenticated reconnect; coarse browser/major version/OS only.
- Consistent generic labels and device icons; unknown client details explicitly shown.
- Fix Web Chat/Terminal ordering and status accessibility labels found in the cross-surface audit.

## Surfaces

- Terminal: existing roster reads the stored label; no new CLI verb or auth change.
- Menubar: label editor, consistent display fallback, platform-aware icons.
- Phone: same roster label and unknown-information wording; device administration remains Mac-only.
- iPad: same shared phone implementation.
- Web: enrollment metadata and Chat/Terminal order in focus/tiles; no device-administration UI.

## Scope

No user-assigned-device-name entitlement or provisioning change; no browser fingerprinting, hardware-model lookup, credential changes, production deployment or release. Existing devices need an authenticated connection to supply missing client metadata.
