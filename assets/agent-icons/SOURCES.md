# agent-icons — provenance

These PNGs are each vendor's **official mark**, committed for **one purpose**: to
identify which coding agent a tmux pane is running inside gtmux's UI (menu-bar /
mobile / web). This is nominative use — they are not gtmux's branding, and gtmux
draws no third-party mark of its own. This directory is the deliberate exception to
"no third-party marks in the repo" — see `CLAUDE.md` §6.

Each icon is provided by the repository owner from the vendor's official source.
Trademarks belong to their respective owners. To update or remove one, replace or
delete `<key>.png`; a missing icon simply falls back to the neutral monogram.

| file (registry key) | agent | owner |
|---|---|---|
| `opencode.png` | opencode | SST / opencode |
| `codex.png` | Codex | OpenAI |
| `gemini.png` | Gemini CLI | Google |
| `grok.png` | Grok | xAI |
| `glm.png` | GLM | Zhipu AI |
| `kimi.png` | Kimi | Moonshot AI |
| `dsh.png` | DeepSeek Harness | DeepSeek |

Notes:
- `grok` and `glm` need registry entries (detection commands) before their panes are
  recognized as agents and the icon is shown; `opencode`, `gemini` and `kimi` are
  already detected.
- `gemini.png` is a wide wordmark (2:1) rather than a square glyph — replace with a
  square icon when available for a cleaner avatar.

`dsh.png` is rasterized at 512×512 from DeepSeek Harness's official whale
[apps/web/public/favicon.svg](https://github.com/deepseek-ai/deepseek-harness/blob/dsh-v0.2.0-rc.2/apps/web/public/favicon.svg)
(as shipped in `@deepseek-ai/dsh` 0.2.0-rc.2). Source SVG SHA-256:
`9e983b4f649c25c6ca0623a50be1a6e705fd8f49f756638b980fa13b40575ab6`.
The paths are unchanged; a white canvas keeps the official black mark legible in
both UI themes. This identifies compatibility and implies no endorsement.
