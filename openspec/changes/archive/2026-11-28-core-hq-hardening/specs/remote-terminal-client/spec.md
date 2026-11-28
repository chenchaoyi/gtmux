## ADDED Requirements

### Requirement: Attach WebSocket liveness deadlines and idle cursor gating

`GET /api/attach` SHALL enforce WebSocket frame size limits, read deadlines refreshed by client frames and pongs, and periodic server `Ping` frames with write deadlines so silently dropped peers on quiet panes close and reap their PTY client. The cursor sampler SHALL skip subprocess calls when `predict=0`, while flow is paused, or after 2 seconds without PTY output or client input.

#### Scenario: Dead attach peer on a quiet pane is reaped

- **WHEN** an attached WebSocket peer stops responding to server `Ping` frames while the pane produces no output
- **THEN** the server closes the WebSocket and terminates and reaps the spawned tmux client process

#### Scenario: Idle or non-predictive attach stops polling cursor

- **WHEN** `gtmux attach` connects without `--predict` (`predict=0`) or an attach session goes idle for over 2 seconds
- **THEN** `AttachCursor` is not invoked until predictive activity resumes
