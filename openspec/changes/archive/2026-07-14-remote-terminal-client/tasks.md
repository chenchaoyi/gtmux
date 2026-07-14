## 1. Deps + wire protocol + target parse (foundation)

- [x] 1.1 Add pure-Go deps: `creack/pty`, `gorilla/websocket`, `golang.org/x/term`; drop the mistaken bubbletea/lipgloss deps; confirm `CGO_ENABLED=0 go build ./cmd/gtmux` passes
- [x] 1.2 `internal/connect` frame codec: encode/decode binary opcode frames (INPUT/RESIZE/PAUSE/RESUME/OUTPUT), payload at index 1; RESIZE `{cols,rows}` JSON. Unit-tested (round-trip, bad frames)
- [x] 1.3 Target parser: `#t=` share link → guest; host + `--token` → owner (reuse from the earlier draft). Unit-tested

## 2. Server — /api/attach WS bridge (internal/server)

- [x] 2.1 `GET /api/attach?id=%N` WS handler: authed; scope gate — refuse the upgrade unless owner OR guest-view-allowed
- [x] 2.2 Spawn `tmux new-session -t <session>` (+ select the pane's window/pane) inside a `creack/pty` PTY; bridge master↔WS in two goroutines, ctx-cancel both on either side closing
- [x] 2.3 Input gating: drop INPUT/RESIZE frames when the caller may not type into the pane (view-only guest); RESIZE → `pty.Setsize`
- [x] 2.4 Flow control: bounded PTY-read buffer; on PAUSE stop reading the PTY, on RESUME continue; guard against deadlock
- [x] 2.5 Tests: scope gate (owner any / guest view-allowed only / refuse otherwise), input-drop for view-only, frame codec on the server side

## 3. Client — `gtmux attach` raw terminal (internal/connect + cmd)

- [x] 3.1 WS-connect to `/api/attach`; put stdin/stdout terminal in raw mode via `x/term`; RESTORE on every exit path (defer + signal)
- [x] 3.2 Pump: stdin → INPUT frames (skip when `--read-only`); OUTPUT frames → stdout raw
- [x] 3.3 SIGWINCH → RESIZE frame with current size; send initial size on connect
- [x] 3.4 Flow control client side: high/low water on unacked output → PAUSE/RESUME; a detach key (e.g. Ctrl-\\ or a prefix) to quit cleanly
- [x] 3.5 Wire `gtmux attach <target> [%pane] [--token --read-only]` into dispatch; en+zh strings + `--help`

## 4. Docs + specs + gate

- [x] 4.1 Capture the research in `docs/design/remote-attach-research.md`; document the frame protocol + `/api/attach` in `api/contract.md`; add `attach` to `docs/cli.md` + CLAUDE.md command list
- [x] 4.2 `make check` + `CGO_ENABLED=0 go build ./cmd/gtmux` green; smoke: `gtmux attach --help`
- [x] 4.3 `openspec validate --specs --strict` passes; archive after merge
- [x] 4.4 Note: interactive attach (raw mode, TUI fidelity, flood/flow-control) is verified MANUALLY on a real terminal against a live serve (documented in the PR)
