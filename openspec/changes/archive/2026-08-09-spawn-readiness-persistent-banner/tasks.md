# Tasks — spawn-readiness-persistent-banner

Status: DONE (approved 2026-08-09, widened with group 4; implemented + specs synced).

## 1. Stop a persistent auth warning from failing the ready gate forever

- [x] 1.1 Mechanism: recategorize — drop `MCP servers need authentication` /
      `need authentication` from `bootBanners` (`internal/prompt/prompt.go:161-162`) and
      record WHY in the code (a standing notice names a user action and never resolves;
      boot noise is what resolves by waiting). The two rejected candidates and the reason
      they collapse are in the proposal.
- [x] 1.2 Test: a capture with a ready composer prompt AND a persistent
      `N MCP servers need authentication · run /mcp` bottom line ⇒ `IsComposerReady` true;
      a genuine boot banner (no composer yet) ⇒ still not ready.
- [x] 1.3 Control test (the boundary): the TRANSIENT sibling still blocks — a
      `Connecting…`/`Loading` bottom line with a composer row present ⇒ NOT ready.

## 2. Make the ready-timeout evidence name the blocker

- [x] 2.1 `prompt.NotReadyReason(capture, agent)` names why a capture is not ready
      (startup gate / the matched boot-banner LINE / a live menu / no composer row);
      `prompt.BootBannerLine` is the diagnostic twin of `hasBootBanner`.
- [x] 2.2 `dispatchbridge.ReadyBlocker(pane, agentCmd)` turns that into the gate's verdict,
      including the two conditions the string predicate cannot see: the agent never
      launched (still a bare shell) and the composer never settled.
- [x] 2.3 `spawn.go:192`'s evidence LEADS with the blocker and carries the capture's bottom
      region (`dispatch.EvidenceTail`), not 200 lines of scrollback — the measured reason
      the `✗ NOT delivered` line was drowned rather than missing.
- [x] 2.4 Test: the timeout evidence contains the matched banner line; `ReadyBlocker`'s
      shape is pinned per condition.

## 3. Consistency (per repo rule)

- [x] 3.1 Update `docs/TROUBLESHOOTING.md` (the footgun costs real time — this is its second
      recurrence) and the KB `pitfalls.md` "fixed 2026-08-01 / #652" note, which is now
      inaccurate for the persistent-banner form.
- [x] 3.2 Sync the spec delta into `openspec/specs/agent-dispatch/spec.md`; archive this
      change once implemented.
- [x] 3.3 `docs/cli.md`: the `gtmux tasks` status list gains `undelivered`.

## 4. A never-delivered dispatch must never render as `done`

The ledger's `Delivered` is written and then read by nobody but the resume lookup
(`internal/dispatch/ledger.go:172`); `gtmux tasks` derives status purely from the pane's
live radar state (`internal/hq/taskscmd.go:41-47` → `radar.TaskStatusFor`, which maps
`idle → done`). A ready-timeout leaves a live, empty, idle pane — so the failed dispatch
renders green `✳ done` with the full goal text beside it. This is independent of group 1:
fixing the gate does not fix this.

- [x] 4.1 Ledger: add an additive `state` field (the dispatch verdict spawn already has)
      and `Task.Undelivered()` — false for `delivered:true` and for `queued` (accepted,
      behind the current turn), true otherwise. `spawn` records the state on all three
      write paths.
- [x] 4.2 `radar.TaskStatusOf(task, paneStatus)` is the single mapper: an undelivered entry
      is `undelivered`; everything else falls through to `TaskStatusFor`. `gtmux tasks`
      and the digest's `task_status` both use it.
- [x] 4.3 `gtmux tasks` renders `undelivered` first (nothing is running — it outranks
      waiting) with its own glyph/label, en+zh.
- [x] 4.4 A rescue that WORKS closes the loop: a `gtmux send` that LANDS in a pane with an
      undelivered ledger entry back-fills `delivered:true` — otherwise the documented
      workaround leaves a permanently-wrong row.
- [x] 4.5 Tests: an undelivered entry on an idle pane is `undelivered` (never `done`); a
      queued one is not undelivered; a delivered one still derives from the pane; the
      send back-fill flips the row; a legacy entry (no `state`) with `delivered:false`
      reads undelivered.
- [x] 4.6 Spec delta: MODIFY "Dispatch ledger and needs-you view" — status derives from the
      pane ONLY for a dispatch that landed.
