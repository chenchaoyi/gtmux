## Architecture
Use build-selected adapters for host resources and desktop integration, and a user-service interface with launchd and systemd implementations. Shared lifecycle orchestration owns enrollment, locks, credentials and route rollback; adapters own artifact rendering, paths and bounded manager calls. Do not spread GOOS conditionals across feature code.

Linux services run as the current user and retain the same HOME/PATH/tmux socket environment. Do not use sudo or enable lingering automatically. Report unavailable user managers before provisioning or changing remote mode. Explain login/logout persistence separately from enabled units. Never report installation as active connectivity.

Linux resource sampling reads procfs and the filesystem containing gtmux state. Memory means MemAvailable/Total; thresholds are explicit. Battery monitoring is not provided in the first Linux server implementation and is marked not applicable, distinct from failed readings.

Desktop focus/activation and lid/server-mode controls remain macOS-specific. Linux restore reconstructs sessions headlessly, then prints attach guidance. Phone push is independent of desktop notification delivery. Each server owns its own pane IDs and HQ.

## Verification
Pure fixture tests for parsers/renderers, isolated fake managers for lifecycle/failures, real private tmux and HTTP on Ubuntu CI, Linux amd64/arm64 cross builds and existing macOS tests. No live account or real HQ mutations. Record real-device and real-systemd limits honestly.
