# Tasks — spawn-readiness-persistent-banner

Status: PROPOSED (ticket only, not started). The commander decides whether/when to fix.

## 1. Stop a persistent auth warning from failing the ready gate forever

- [ ] 1.1 Decide the mechanism (see proposal): drop the `MCP servers need authentication` /
      `need authentication` phrases from `bootBanners` (`internal/prompt/prompt.go:161-162`),
      OR gate them on the composer prompt being absent, OR treat a banner that survives the
      two-frame settle as chrome. A still-booting pane must still be caught; a settled
      composer that merely carries a permanent auth notice must read as READY.
- [ ] 1.2 Test: a capture with a ready composer prompt AND a persistent
      `N MCP servers need authentication · run /mcp` bottom line ⇒ `IsComposerReady` true;
      a genuine boot banner (no composer yet) ⇒ still not ready.

## 2. Make the ready-timeout evidence name the blocker

- [ ] 2.1 When the ready gate times out, include the specific `hasBootBanner` line that
      matched in `spawn.go:192`'s evidence (not just "composer not ready within the ready
      timeout"), so a persistent-banner block is not misread as "agent slow".
- [ ] 2.2 Test: the timeout evidence contains the matched banner line.

## 3. Consistency (per repo rule)

- [ ] 3.1 Update `docs/TROUBLESHOOTING.md` (the footgun costs real time — this is its second
      recurrence) and the KB `pitfalls.md` "fixed 2026-08-01 / #652" note, which is now
      inaccurate for the persistent-banner form.
- [ ] 3.2 Sync any spec delta into `openspec/specs/agent-dispatch/spec.md`; archive this
      change once implemented.
