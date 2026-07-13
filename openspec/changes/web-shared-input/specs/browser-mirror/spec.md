# browser-mirror Specification

## MODIFIED Requirements

### Requirement: Web UI served by `gtmux serve` — view, plus consented per-pane input

`gtmux serve` SHALL serve a self-contained web UI at `GET /`, embedded in the binary
(`//go:embed`, no build step, offline-safe, cgo-free) and served same-origin as the
existing `/api/*`. The UI SHALL present the agent radar and a pane mirror, using the
shared status language (color+shape+glyph; section order waiting→working→idle→running)
identical to the other surfaces. The UI is READ-ONLY BY DEFAULT. It MAY additionally
expose a terminal-input affordance, but ONLY for panes the caller is authorized to type
into, which it SHALL learn from `GET /api/share` (`{input, panes}` for the caller) and
NEVER assume: no input control is shown for a pane the server does not authorize, and
any typing goes through `POST /api/send`, whose server-side gate is authoritative. A
guest whose host has not consented, or a disallowed pane, shows no input control.

#### Scenario: Browser loads the web UI

- **WHEN** a browser requests `GET /` from a running `gtmux serve`
- **THEN** the embedded web UI is returned and renders the agent radar after pairing

#### Scenario: Input shown only for authorized panes

- **WHEN** the UI has fetched `GET /api/share` for the caller
- **THEN** an input affordance is shown ONLY for the panes it lists; other panes stay
  view-only, and a guest with input off sees no input control anywhere

#### Scenario: The server gate, not the UI, is authoritative

- **WHEN** a guest `POST`s `/api/send` for a pane not in its authorized set
- **THEN** the send is refused server-side regardless of the UI state
