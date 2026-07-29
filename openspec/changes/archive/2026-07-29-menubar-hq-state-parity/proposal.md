# Menu-bar HQ card: full state parity with the mobile disc

## Why

The mobile app renders HQ as a floating disc whose **ring color + badge** encode a
six-state model at a glance (absent / needs-your-call / worker-needs-you / resource /
working / normal — MOBILE §17). The menu-bar renders HQ as a card that is a good
meta-layer (role banner + framed panel + intelligence headline) but expresses state
only weakly: DESIGN §12 v2 deliberately made the card BADGE-LESS ("HQ 恒在监视, 无
idle/working 之分"), encoding just the hqWaiting case via an amber card border. So the
two surfaces show the same supervisor with two different visual languages, and the
menu-bar can't tell working from normal from a resource bottleneck at a glance.

The commander asked to unify the surfaces. A floating disc doesn't fit a fixed popover,
but the disc's IDENTITY TOKEN does: make the card's avatar the same circular HQ
medallion and give it the disc's full ring/badge state model. This revisits the §12 v2
"no badge" decision on purpose — the two surfaces now share one HQ visual language.

## What Changes

- **A shared HQ medallion** replaces the flat brand-grid avatar on the HQ card: a circle
  with the brand mark + "HQ" wordmark + a state ring, visually identical to the mobile
  disc.
- **Full state model on the card**, resolved by a pure Swift resolver mirroring the
  mobile `discState` (same priority): red ring + `!` (HQ waiting) > red + count (a worker
  waiting) > red + `⚠` (machine at the RED resource tier) > cyan (HQ working) > green
  (normal). The intelligence headline stays as the subtitle.
- **A slow `gtmux resource --json` poll** in the menu-bar (it currently polls only
  `agents --json`) to source `machine.tier` for the resource state — cheap (df/sysctl),
  reddens ONLY on `tier == "red"`, never a soft amber (低噪, matching mobile).
- **DESIGN §12 updated** to describe the state medallion (superseding the v2 "no badge"
  wording), and MOBILE §17 gains a cross-surface-parity note. The `menu-bar-app` spec's
  HQ-card requirement is MODIFIED accordingly.
- The absent state keeps its menu-bar behavior (a start affordance that shells
  `gtmux hq` — the phone can't spawn one) but renders as the same dimmed grey medallion.

## Impact

- Affected specs: `menu-bar-app` (MODIFY the HQ-card requirement).
- Affected code: `macapp/Sources/GtmuxBar/` — `AgentStore` (resource poll + HQ-state
  resolver), a new medallion view in `Components.swift`/`MenuView.swift`, `MenuView.hqCard`.
- Affected docs: `docs/design/DESIGN.md` §12, `docs/design/MOBILE.md` §17.
- No contract or CLI change (reuses the existing `gtmux resource --json`). Menu-bar only;
  the mobile disc is unchanged (it is the reference the card now matches).
