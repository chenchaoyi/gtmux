# Resilient surface polling

## Why

Slow HQ and terminal reads can overlap and let an older response replace newer data.
Other native/Web periodic reads have no execution bound. A stalled poll must not
freeze updates or accumulate processes, and switching scope must retire its replies.

## What changes

Use reusable, per-resource read lifecycles with one operation at a time, cancellation,
deadlines and scope retirement. Keep the last valid snapshot on read failure. Bound
periodic native CLI work and release its gate on every exit. Preserve mutation receipts
and the ordered server/HQ sensors. Codex producer indexing is a separate follow-up.

## Surfaces

- 终端 (terminal / attach): no protocol or capture changes; native periodic tab-order reads become bounded.
- 菜单栏 (menubar): periodic reads cannot accumulate; failed snapshots retain valid data.
- 手机 (phone): HQ and terminal polling cannot apply obsolete responses or remain stuck forever.
- iPad: shares the phone read lifecycle.
- Web: periodic reads are coalesced and bounded within their view/credential scope.

## Non-goals

No global transcript/status cache, parallel HQ sensor execution, automatic retries of
mutations, release/install, or new terminal history behavior.
