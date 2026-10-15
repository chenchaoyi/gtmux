## Why
The cgo-free core compiles for Linux, but macOS-only installation, service control and sampling prevent a complete server workflow. Client copy also incorrectly calls every server a Mac.

## What Changes
- Support Linux amd64/arm64 servers: installation/update, agent hooks, tmux/HQ, authenticated Web/mobile access and optional systemd user services.
- Isolate platform behavior behind service, resource and desktop integration adapters. Preserve launchd/macOS behavior; unsupported Unix systems fail honestly.
- Use neutral host language in shared UI and documentation. Preserve actual macOS-only feature names.
- Add Linux runtime coverage, portable installer tests and bilingual operating guidance.

## Surfaces
- terminal: Linux lifecycle, diagnostics, resources and headless tmux control.
- menubar: remains a local macOS application; shared connection terminology is neutral.
- phone: Linux pairing, session management, host information and platform capability presentation.
- iPad: same server behavior through the shared mobile implementation.
- Web: Linux-hosted sessions and neutral host language; desktop-only actions report unsupported.

## Scope
Linux first (systemd for optional background services). Foreground serve/tunnel needs no systemd. FreeBSD and cross-host HQ federation are not delivered. No release, production configuration or existing session changes.
