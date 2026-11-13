# Restore server sessions from the App

[中文](remote-session-restore.zh.md)

Open Servers, choose a paired owner server’s More menu, then Restore sessions. Review the backup time and missing sessions. Starting recovers the saved layout and follows the displayed agent setting; it sends no new task. Already live/detached sessions do not enter the remote startup scope. The server uses its normal trusted tmux-resurrect installation and configured hooks; this is an owner capability, not a sandbox for untrusted backups.

The page shows preparation, layout recovery, agent startup, and separate confirmed/pending/skipped results. It scrolls on phone/tablet and supports large text. You may leave it; reopening reads the persisted server job. Only explicit Start or Retry submits a mutation. A lost response retains the same ID locally. Switching servers retires old callbacks before another target can post. Guests and demo users cannot restore. An older server asks for an update instead of displaying an empty plan.

The core reserves a durable private receipt before starting one detached child. That child uses the same process-owned execution lease as CLI/menu restore, rechecks the preview, restores layout with agent startup off, verifies the approved inputs again, then runs the existing agent producer only for the approved missing sessions and identities. It rechecks the current pane locator and shell before sending. No local terminal is opened by the remote worker. Failures and incomplete startup remain visible; nothing automatically relaunches after interruption. The native menu keeps its existing progress display rather than applying its generic action close. Explicit Escape/outside dismissal still wins.

The owner API is in api/contract.md. It omits private commands/paths/goals. IDs age out for new admission after24h, and terminal receipts stay48h, capped128; capacity/IO errors fail closed. All CI/local evidence uses isolated HOME, synthetic tmux and network seams. Real reboot/session restoration and installed App interaction require separate acceptance; implementation tests do not perform these actions.

The screenshot of a local frontend403 alone cannot establish a Codex session quota. The native bridge returns403 for hard process/home/TTY/identity refusals and503 for unavailable observations/deadlines. Inspect the matching `codex.frontend.startup.*` log reason and timestamp before choosing a remedy; an old pane’s restored screen may predate current startup logs. Never automatically retry a hard refusal or replay user tasks.
