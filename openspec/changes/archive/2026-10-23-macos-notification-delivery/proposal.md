# Reliable Mac notification delivery

The menu-bar consumer remembers only the last kind per pane. A successful completion can therefore suppress later turns indefinitely, and a failed submission poisons the same cache. Queue setup also has a drain-before-watch gap, and hide-when-idle skips withdrawal. Existing logs confuse acceptance by macOS with a visible banner.

Remove the lifetime kind cache and let each distinct queued event request delivery. Keep one current banner per pane/kind using system identifiers, withdraw resolved waits, watch before draining, and reconcile the queue during normal polls. Record refusals and system acceptance honestly. Surface system authorization and a deliberate, content-free test in the existing compact notification preferences. Keep ordinary input alerts active and successful turn completion silent, respecting Focus; do not add new event classes or disclose transcript text.

## Surfaces

- Terminal: existing hook producers retain their lifecycle gates; completion copy describes a turn, not completion of the entire task.
- Menubar: reliable queue delivery, native banner behavior, compact system status/test/settings controls.
- Phone: APNs timing, consent and UI unchanged; this is the local Mac delivery path.
- iPad: same boundary as phone.
- Web: shared agent lifecycle unchanged; no browser notification feature added.

## Validation

Use an injected notification-center adapter and private temporary queue to exercise repeated turns, asynchronous failures, resolved waiting banners, stale/off/denied states and explicit test requests without touching real sessions. Run core and native gates, then exact-head hosted CI. System permission/banner/Focus acceptance remains a separate live-system boundary when UI inspection is unavailable.
