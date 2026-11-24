# gtmux design docs

The terminal, menu bar, phone, iPad and browser use the same status colors, shapes
and symbols. Update both languages of a current design in the same PR.
`scripts/check-design.sh` lists the exceptions for single-language records.

## Where to start

| File | What it is for |
| --- | --- |
| `SURFACES.md` | The five surfaces: the checklist every proposal walks through, and the four structural rules that keep one surface from drifting away from the others. |
| `DESIGN.md` | The menu-bar app's authority, and §0–§3 the status language every surface shares. |
| `MOBILE.md` | The phone and iPad authority (app icon / agent icons / interactions / push / states); §5 is the iPad, the same app's regular shell. |
| `WEB.md` | The browser mirror's authority (workbench, the read-only line, chat mode, avatars, keyboard). |
| [Knowledge layers](knowledge-layers.md) | The managed charter, user rules and local ledger. Covers who writes them, how they are loaded and how lessons reach other audiences. |
| [State storage](state-storage-lifecycle.md) | Managed directories, offline grouping, recovery and rollback. |
| `hq-move-between-macs.md` | Implemented move to a new Mac: preview, stage and review selected knowledge and user instructions; keep the old board and generated instructions out of the migration. |
| `knowledge-engineering-research.md` | The survey behind the knowledge engine: nine practices compared, what was borrowed and what was not. |
| `terminal-keyboard-research.md` | Shared terminal-key input: mobile terminal precedents, function/modifier keys, a Keys panel, compatibility and acceptance boundaries. Implementation and acceptance limits included. |
| [libghostty research](libghostty-research.md) | The 2026-11-10 native/Web PoCs, pinned versions, renderer failures and paired measurements; why replacement was rejected and what would justify another evaluation. |
| [Agent integration map](../guides/agent-integrations.md) | Every built-in agent: discovery, events, history, input, installation and limits, with implementation links. |
| `agent-onboarding.md` | How to add or iterate a coding agent: support tiers, the registry as the single source of identity, the step list and the pitfalls. |
| `HANDOFF.md` | Acceptance criteria and order of work from the original design handoff. Use the current surface specification for a new change. |
| `SECURITY.md` | The security posture and its boundaries. |
| `remote-access-tunnel.md`, `remote-attach-research.md`, `server-mode-research.md`, `multiplexer-research.md`, `multi-agent-multi-terminal.md`, `mosh-predictive-echo-research.md` | Research behind decisions, kept so the next iteration reads the record instead of re-deriving it. |
| `ITERATIONS-2026-06.md`, `REVIEW-mobile-01.md`, `AUDIT-2026-09-07.md`, `HANDOFF-mobile-2026-06.md`, `DECISIONS-FOR-CCY.md`, `RESEARCH-prior-art-2026-06.md` | Dated records of one round or one review. History; not updated. |
| `mockup/gtmux-menubar.dc.html`, `mockup/gtmux-mobile.dc.html`, `mockup/gtmux-web.dc.html` | Interactive prototypes (open in a browser; they load their runtime from the network). |
| `mockup/preview-*.png` | Static references for the menu bar. |

Before changing a UI, read its current design and acceptance criteria. Update the
design and the other affected surfaces when a change adds or alters a shared behavior.

[DeepSeek Harness terminal clients](dsh-terminal-clients.md) — native client adapters, addressed input and explicit message review.
