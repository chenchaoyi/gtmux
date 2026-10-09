# Per-conversation HQ follow for desktop Codex

## Why
Desktop Codex work conversations currently count toward attention and notifications and are read by digest/mining without a separate enrollment decision. Detection should not imply supervision or learning.

## What changes
Verified desktop Codex conversations default to status-only. A local, durable per-session policy opts into HQ observation, with separate notification and knowledge permissions. Settings are owner-only, keyed by the verified conversation ID on this Mac. Stopping follow clears notification and learning permissions; re-enrollment starts a new observation interval. No desktop control/adoption is added. Existing terminal and other-agent behavior is retained.

## Surfaces
- CLI / terminal: `follow` reads or changes one verified desktop conversation; digest and HQ event reads honor its observation interval.
- menubar (menu bar): subdued desktop section, current policy badge, per-session settings panel and managed-count exclusion.
- phone: session settings from the row, save/error feedback, independent off-by-default choices.
- iPad: the same RadarPanel/settings component within the regular shell, bounded sheet width.
- Web: status-only desktop rows and managed-count exclusion; owner can use CLI for settings. No guest follow control.

## Authority
HQ follow permits reading and reporting only. Notification opt-in does not permit input. Knowledge opt-in permits future transcript mining, not automatic publication of lessons. Raw lifecycle metadata remains available for diagnosis. Existing knowledge is not removed by changing these settings.
