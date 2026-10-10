# Function icon inventory · 2026-10-10

[中文](ICON-PARITY-2026-10-10.zh.md)

| Function | Mac menu bar | iPhone / iPad | Web / terminal | Result |
|---|---|---|---|---|
| Knowledge base | HQ shortcut: SF `book` | HQ menu: `SIcon knowledge`, open book | No Web KB shortcut; CLI uses names | Fixed Mac diamond |
| Situation board | SF `doc.plaintext` shortcut, named report row | Named HQ row | No corresponding reader shortcut | Clear document metaphor; text rows stay text |
| Usage | Named HQ report row | HQ menu: bar chart | CLI names; no Web usage shortcut | Consistent meaning; no decorative icon added |
| Network route | Named region rows in Preferences / Pair | Settings: `SIcon globe` | CLI names | Fixed mobile server icon; Mac identity keeps server icon |
| All panes | SF `rectangle.split.2x2` | `PanesIcon`, split window | Web split-window SVG | Fixed Web document character |
| New session | Plus | `NewSessionIcon`, plus in a terminal outline | CLI command; no Web creator | Same creation metaphor |
| General settings | SF `gearshape` | `SettingsIcon`, gear | Web `Aa` opens font/size only | Keep different icon for narrower appearance control |
| Notifications | Bell | `SIcon bell` / `bellOff` | CLI names | Same metaphor |
| Language | Globe | `SIcon globe` | Browser language, no selector | Same metaphor where selectable |
| gtmux identity | Top-right cyan pane, wide bottom pane | Shared app/HQ mark; widget BrandIcon asset | Web pane mark; CLI text | Existing brand geometry retained |
| Agent identity | Official agent marks from registry/assets | Shared `AgentAvatar` for phone and iPad | Web icon endpoint | No alternate agent artwork introduced |

## Sources and verification

- Mac: `MenuView.swift`, `HQReader.swift`, `Preferences.swift`, `RemoteAccessControls.swift`, `Theme.swift`, `BrandMarkTests.swift` under `macapp/`.
- Phone/iPad: shared `SettingsScreen.tsx`, `HQDisc.tsx`, `SettingsIcons.tsx`, `SettingsIcon.tsx`, `Icons.tsx`, `BrandMark.tsx`, `AgentAvatar.tsx`; widget `GtmuxWidget.swift` / `BrandIcon.imageset`.
- Web: `internal/server/web/index.html`, `app.js`, `style.css`; official agent artwork: `assets/agent-icons/SOURCES.md`.

This is a source-level inventory plus automated checks, not a claim of physical iPhone/iPad layout or VoiceOver acceptance. Text-only surfaces need no new icons. Native stroke/shape details may differ while preserving the same metaphor.

Validation results are recorded in the PR: native SF-symbol availability and reader tests, mobile route/shortcut checks, bilingual Web accessible-name checks, and repository gates.
