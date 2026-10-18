# Local memory diagnostics

## Why
An abnormally large Go serve process cannot currently explain its memory use beyond OS footprint. A new process cannot recover the old process's heap, and adding a public pprof endpoint would expose an unrelated access surface.

## What Changes
- Add an explicit foreground `serve --memory-diagnostics DIR` option, off by default.
- Serve allocator counters and sampled heap/allocs profiles on a private Unix socket, separate from phone/browser routes.
- Bound capture admission and encoded output; preserve GC/sampling settings, fail closed on unsafe paths, and clean up only this run's socket.
- Document local capture, interpretation and lifecycle limits. This adds diagnostic capability, not a memory-leak fix.

## Impact
Go CLI only; no native build, API contract change, service installation change, default listener or periodic sampler. Production activation requires a separately reviewed service transition.

## Surfaces
- Terminal: explicit foreground diagnostic flag, local startup warning/path, and maintainer capture documentation.
- Menubar: no UI or service-installation behavior change.
- Phone: no diagnostic routes or controls; existing API remains unchanged.
- iPad: same unchanged API boundary as phone.
- Web: no diagnostic routes; local Unix transport is separate from the browser listener.
