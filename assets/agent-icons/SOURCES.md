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

Notes:
- `grok`, `glm`, `kimi` need registry entries (detection commands) before their
  panes are recognized as agents and the icon is shown; `opencode` and `gemini` are
  already detected.
- `gemini.png` is a wide wordmark (2:1) rather than a square glyph — replace with a
  square icon when available for a cleaner avatar.
