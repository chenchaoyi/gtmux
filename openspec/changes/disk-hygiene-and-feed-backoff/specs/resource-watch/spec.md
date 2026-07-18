# resource-watch — delta

## ADDED Requirements

### Requirement: gtmux bounds its own on-disk footprint

The system SHALL bound the disk it consumes for its own logs and upload sink, so a
long-running install cannot fill the volume with its own output. On the serve slow-tick,
gated to run at most once per 30 minutes, gtmux SHALL:

- **Cap the launchd logs.** The always-on `gtmux serve` / tunnel LaunchAgents log to
  `~/.local/share/gtmux/{serve,tunnel,selftunnel,restore}.log`, which launchd NEVER
  rotates. When such a log exceeds a maximum size, gtmux SHALL truncate it to only its most
  recent tail (starting on a clean line boundary), so the file cannot grow without limit
  while the `O_APPEND` writer keeps appending.
- **Prune the uploads sink.** The phone-upload directory `~/.local/share/gtmux/uploads/`,
  written on every `/api/upload`, SHALL be pruned: entries older than the retention window
  are deleted, and if the directory still exceeds a total-size cap, the oldest entries are
  deleted until it is under the cap.
- **Age out dead-pane churn markers.** The per-pane ephemeral marker dirs (`frame/`,
  `cpu/`, `goalchanged/`, `sends/`) accumulate a file per pane and never clean up a dead
  pane's leftover. gtmux SHALL delete markers older than a staleness cutoff; a LIVE pane's
  marker is refreshed each sample so its mtime stays fresh and it survives. The digest /
  idle-since sources (`resume/`, `usage/`, `usagewarn/`) SHALL NOT be aged out.

The sweep SHALL be best-effort (a missing path or an I/O error is a no-op that does not
disturb the rest of the tick) and SILENT (housekeeping, not a perception event — it emits
no HQ nudge).

Additionally, `gtmux doctor` SHALL surface a `Storage` row reporting the total gtmux
state-dir footprint, flagging it amber past a soft threshold and red past a hard one, so a
retention breach (typically a runaway unrotated log) is legible before the disk fills.

#### Scenario: An over-cap launchd log is trimmed to its tail

- **WHEN** `serve.log` has grown past the maximum size and the hygiene sweep runs
- **THEN** the file is shrunk to its most recent tail (bounded), starting on a clean line

#### Scenario: Stale uploads are pruned

- **WHEN** the uploads dir holds files older than the retention window, or its total size
  exceeds the cap
- **THEN** the old files (and, past the size cap, the oldest files first) are deleted

#### Scenario: A dead pane's stale marker is aged out, a live pane's is kept

- **WHEN** a churn-marker dir holds a stale marker (a dead pane, old mtime) alongside a
  fresh one (a live pane, recently refreshed)
- **THEN** the stale marker is deleted and the fresh one survives

#### Scenario: A fresh, small footprint is left alone

- **WHEN** the logs are under the cap and the uploads/marker dirs are small and recent
- **THEN** the sweep deletes and truncates nothing

#### Scenario: The doctor flags a runaway footprint

- **WHEN** the gtmux state dir grows past the hard threshold (a runaway unrotated log)
- **THEN** `gtmux doctor`'s `Storage` row reports it red, pointing at the likely log
