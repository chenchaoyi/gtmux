# Accept known backend sources for verified Codex TUI creation

## Why
A Codex 0.165 TUI conversation can record `originator=codex-tui` with
`source=vscode`. Requiring `source=cli` rejects an otherwise verifiable terminal
client. Creation provenance must remain separate from current ownership.

## What Changes
- Accept `cli` and `vscode` only with explicit TUI creation provenance.
- Retain exact foreground process, stdin, birth, home, pane and ambiguity checks.
- Document that a shared daemon PID still cannot identify its frontend.

## Surfaces
- terminal: compatible process recovery; no input or protocol change.
- menubar: consumes the verified core identity.
- phone: consumes the verified core identity.
- iPad: consumes the verified core identity.
- browser: consumes the verified core identity.
