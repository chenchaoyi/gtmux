# Codex terminal identity across embedded and shared servers

## Why
Shared Codex backends reuse a PID and inherited terminal environment across independent TUI clients. A new thread can therefore lose its pane association; tools can report another pane or borrow HQ authority. Backend lifetime also leaves minimal startup observations after a client closes.

## What changes
Observe successful root-thread operations on each kernel-verified local frontend connection. Keep legacy independent-client process verification. Route shared input to the exact home/thread with durable no-replay claims; retain draft, waiting and authority guards. Use executing tool thread identity for reports, HQ authorization, audit attribution and HQ event reads. Add reversible ordinary-shell integration and preserve custom launchers and unsupported options. Permit guarded busy HQ input and use consistent pending-action typography.

## Surfaces
- terminal: new `gtmux codex`, managed interactive Bash/zsh integration, spawn/HQ/restore use the same adapter; explicit legacy and custom launches retain native behavior.
- menubar: consumes core identity/receipts; HQ pending actions share caption typography and concise labels.
- phone and iPad: existing authorized send routes receive exact queued/refused/uncertain results; no local Mac URI or new UI ownership inference.
- web: same radar and input policy for session/trace views; no frontend PID or originator guesses in JavaScript.

## Boundaries
No daemon restart, remote-control enablement, lock manipulation, history deletion, takeover, automatic replay after uncertain delivery, or changing an already-running client. Official protocol/source evidence and actual acceptance boundaries are documented in the paired design note.
