# gtmux design docs

The design authority for one product with five surfaces (terminal incl. remote attach ·
menu bar · phone · iPad · web), all sharing one status language (colour + shape + glyph).
Every file here is a pair: `<name>.md` in English, `<name>.zh.md` in Chinese, both kept
current in the same PR (dated logs are single-language; `scripts/check-design.sh` lists them).

## Where to start

| File | What it is for |
| --- | --- |
| `SURFACES.md` | The five surfaces: the checklist every proposal walks through, and the four structural rules that keep one surface from drifting away from the others. |
| `DESIGN.md` | The menu-bar app's authority, and §0–§3 the status language every surface shares. |
| `MOBILE.md` | The phone and iPad authority (app icon / agent icons / interactions / push / states); §5 is the iPad, the same app's regular shell. |
| `WEB.md` | The browser mirror's authority (workbench, the read-only line, chat mode, avatars, keyboard). |
| `knowledge-layers.md` | The three layers of knowledge (factory charter / your rules / this machine's ledger): who writes each, when it reaches whose head, how an entry moves up. |
| `knowledge-engineering-research.md` | The survey behind the knowledge engine: nine practices compared, what was borrowed and what was not. |
| `agent-onboarding.md` | How to add or iterate a coding agent: support tiers, the registry as the single source of identity, the step list and the pitfalls. |
| `HANDOFF.md` | The order of landing and the acceptance checks for a design round. |
| `SECURITY.md` | The security posture and its boundaries. |
| `remote-access-tunnel.md`, `remote-attach-research.md`, `server-mode-research.md`, `multiplexer-research.md`, `multi-agent-multi-terminal.md`, `mosh-predictive-echo-research.md` | Research behind decisions, kept so the next iteration reads the record instead of re-deriving it. |
| `ITERATIONS-2026-06.md`, `REVIEW-mobile-01.md`, `AUDIT-2026-09-07.md`, `HANDOFF-mobile-2026-06.md`, `DECISIONS-FOR-CCY.md`, `RESEARCH-prior-art-2026-06.md` | Dated records of one round or one review. History; not updated. |
| `mockup/gtmux-menubar.dc.html`, `mockup/gtmux-mobile.dc.html`, `mockup/gtmux-web.dc.html` | Interactive prototypes (open in a browser; they load their runtime from the network). |
| `mockup/preview-*.png` | Static references for the menu bar. |

Before changing any UI, read the authority for that surface and follow it; a deliberate
deviation is proposed first and written back into the document, never left silent.
