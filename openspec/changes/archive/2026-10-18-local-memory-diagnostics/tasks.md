## Tasks
- [x] Implement explicit opt-in, private Unix socket lifecycle and optional failure.
- [x] Add timestamped allocator counters and bounded sampled profiles without GC intervention.
- [x] Verify isolated access, output budgets, concurrency, cancellation and cleanup; demonstrate regression sensitivity.
- [x] Verify synthetic profile parsing and cgo-free build; document capture and limitations.
- [x] Run the core gate and prepare the implemented specification for archive.

## Acceptance
No real serve process is restarted or diagnosed through the new endpoint during implementation. The original large process's retained objects remain unlocated; installation, production capture and field acceptance are separate pending decisions.
