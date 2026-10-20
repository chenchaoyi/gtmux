## Why

DSH conversations are visible but their native token usage is absent. Account
balances and subscription windows have different semantics. Installed agents,
recorded tiers and expired snapshots must not become a hard-coded two-agent view.

## What Changes

- Fold DSH v4 native usage incrementally, following settlement replacement/retry semantics, into session and daily accounting.
- Observe DeepSeek Account balance through its native Cordis service, transporting only sanitized amounts and opaque account identity; no credential copy or inference.
- Add modular usage-source availability and freshness metadata; show recorded plan tiers only with evidence, hide irrelevant sources and retain historical token totals.
- Present balances separately from quota windows and give each source its own observation time. Stale windows do not drive the tightest-window summary or alerts.

## Boundaries

API-key/custom-provider balances require a separately verified provider adapter and remain explicitly unsupported. A logged-in DSH account is not proof of a paid tier. Codex rollout plan tiers are recorded evidence, not a live billing check. No model requests, credential relocation, access-policy or user configuration changes on read.

## Surfaces

- terminal: shared usage JSON, local token accounting, balance and freshness readout.
- menubar: consumes core quota summary; stale windows do not raise fresh quota alerts.
- phone: dynamic usage/plan/balance groups with native DSH identity.
- iPad: same shared sheet and bounded layout.
- Web: consumes the additive shared API; no existing quota page; no new input authority.
