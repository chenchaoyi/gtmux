# Tasks — Menu-bar HQ card: full state parity with the mobile disc

- [x] 1. `AgentStore`: add a `machineTier` (`"" | "amber" | "red"`) published property + a
      slow (~25s) poll of `gtmux resource --json` parsing `machine.tier`; keep it separate
      from the fast `agents` refresh (resource changes on a machine cadence).
- [x] 2. `AgentStore`: add a pure `HQState` enum (`absent/hqCall/needsYou/resource/working/
      normal`) + a `hqState(supervisor:waiting:resourceCritical:)` resolver mirroring the
      mobile `discState` priority; expose `var hqState` on the store. `resourceCritical` =
      `machineTier == "red"` only.
- [x] 3. `Components.swift`: add an `HQMedallion` view — a circle (brand mark + "HQ"
      wordmark) with a state RING (grey absent / cyan working / green normal / red
      attention, from `Theme.Status`) and an optional corner BADGE (`!` / count / `⚠`),
      matching the mobile HQDisc atoms.
- [x] 4. `MenuView.hqCard`: replace the flat `GtmuxLogo` avatar with `HQMedallion` driven
      by `store.hqState`; keep the role banner, framed panel, and intelligence headline;
      the absent slot renders the dimmed grey medallion + the existing `gtmux hq` start
      affordance. Migrate the hqWaiting cue from the amber card border to the medallion's
      red ring (parity with the disc); chrome stays neutral otherwise.
- [x] 5. Colors: ring/badge use the authoritative `Theme.Status` values only (DESIGN §9);
      add NO new colors. Verify against the existing hex-consistency test.
- [x] 6. Docs: update DESIGN §12 (the state medallion supersedes the v2 "no badge"
      wording — describe the ring/badge model + the resource poll); add a cross-surface
      parity note to MOBILE §17 (en+zh where the doc is bilingual).
- [x] 7. Tests: `macapp` `ModelTests` — pin the `hqState` resolver (each state by
      priority, matching `HQDisc.test.tsx`), and that a soft `amber` tier does NOT reach
      the resource state; assert the medallion ring colors come from `Theme.Status`.
- [x] 8. `cd macapp && swift build -c release && swift test` green; `scripts/check-design.sh`
      green (`openspec validate --specs --strict`, palette).
- [x] 9. sync-specs + archive-change (the MODIFY folds into `menu-bar-app/spec.md`).
