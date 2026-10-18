# Local Go memory diagnostics

[中文](memory-diagnostics.zh.md)

`gtmux serve --memory-diagnostics DIR` adds a private local observation socket to
that foreground process. It is off by default: no extra listener or periodic
sampler. It cannot be combined with `--service` or `--unservice`. Enabling it on an
existing managed service requires a planned process transition; the new process
cannot recover the old process's heap. This capability is not a leak fix.

## Start and collect

Use an existing absolute directory owned by the service user, mode 0700, with no
leaf symlink or non-sticky group/world-writable ancestor. Keep the path short
(Unix socket path lengths are limited, especially on macOS). For an **isolated
foreground service** whose bind address and port have already been chosen:

```sh
umask 077
memory_dir=$(mktemp -d /tmp/gtmux-memory.XXXXXX)
gtmux serve --bind 127.0.0.1 --port 18765 --memory-diagnostics "$memory_dir"
```

The startup message prints `DIR/mem-PID-RANDOM/memory.sock`. The new subdirectory
is 0700 and the socket is 0600. The main HTTP server's bind/port are unchanged;
there are no memory routes on the phone/browser API, no TCP debug listener, and
no registration on the default HTTP mux. An unsafe directory or listen failure
prints a warning and leaves the main service able to start, with diagnostics off.
Permissions isolate other ordinary users, not the same account/root; mode checks
are not an ACL attestation. Use a private local directory without inherited ACL
grants. Profiles contain sampled allocation stacks, symbols and source paths,
not object contents; still keep them local and out of Git, messages and uploads.

In another terminal, use the exact printed socket path. Save files inside the
private directory, with a private umask and unique names; do not overwrite
previous evidence:

```sh
umask 077
memory_socket=/absolute/private/directory/mem-PID-RANDOM/memory.sock
capture_dir=$(mktemp -d /tmp/gtmux-capture.XXXXXX)
curl --fail --silent --show-error --max-time 15 --unix-socket "$memory_socket"   http://localhost/stats > "$capture_dir/stats-before.json"
curl --fail --silent --show-error --max-time 15 --unix-socket "$memory_socket"   -D "$capture_dir/heap.headers" http://localhost/heap > "$capture_dir/heap.pprof"
# Wait at least five seconds before requesting any next profile.
curl --fail --silent --show-error --max-time 15 --unix-socket "$memory_socket"   -D "$capture_dir/allocs.headers" http://localhost/allocs > "$capture_dir/allocs.pprof"
curl --fail --silent --show-error --max-time 15 --unix-socket "$memory_socket"   http://localhost/stats > "$capture_dir/stats-after.json"
go tool pprof -top -sample_index=inuse_space "$capture_dir/heap.pprof"
go tool pprof -top -sample_index=alloc_space "$capture_dir/allocs.pprof"
```

Check each curl exit status and HTTP status before accepting a file as evidence.
The endpoint does not save profiles itself. Counters are JSON with `PID`, `Version`,
`GoVersion`, `StartedAt` (diagnostic listener start), `CapturedAt`, `HeapAlloc`,
`HeapInuse`, `HeapIdle`, `HeapReleased`, `HeapSys`, `Sys`, `TotalAlloc`, `HeapObjects`,
`Mallocs`, `Frees`, `NextGC`, `LastGC`, stack/runtime metadata sizes, `NumGC`,
`NumForcedGC`, `NumGoroutine` and `MemProfileRate`. Sizes are bytes; `LastGC` is Unix
nanoseconds. Profile response headers include `X-Gtmux-PID`, `X-Gtmux-Version`,
`X-Gtmux-Captured-At` and `X-Gtmux-NumGC`; save these beside each profile.

## Interpret without changing the workload

[Go's runtime documentation](https://pkg.go.dev/runtime#MemStats) defines the
counters. `HeapAlloc` includes objects still reachable and garbage not yet swept.
`HeapInuse - HeapAlloc` estimates size-class fragmentation; `HeapIdle - HeapReleased`
shows idle heap memory the runtime retains. `HeapSys` is reserved heap address
space, not resident physical memory. `Sys` also includes stacks and runtime
metadata. Compare these with simultaneous OS RSS/footprint/compression readings;
none of those OS measures alone is the Go live heap.

[Go's pprof documentation](https://pkg.go.dev/runtime/pprof#Profile) defines sampled
heap profiles relative to completed GC; the underlying
[MemProfile API](https://pkg.go.dev/runtime#MemProfile) may lag up to two GC cycles.
`alloc_space` is cumulative allocation, while `inuse_space` estimates retained
sampled bytes at that collection view. High allocation totals alone are not a
leak. Compare the same process over time, retain the GC counters, and wait for
natural GC cycles before concluding that an allocation site stays retained.
Correlate growing stacks with bounded, known read-only workloads and application
lifetimes. A forced-GC experiment belongs in an isolated fixture and must be
labelled as an intervention, never substituted for production's unmodified state.

## Limits and cleanup

Only GET `/stats`, `/heap` and `/allocs`, without queries or bodies, are accepted.
There is no GC, goroutine, trace, CPU profile or command-line endpoint. No GC or
sampling setting is changed. Busy captures return 429 instead of queuing;
profiles share a five-second minimum admission interval. More than 16384 sampled
records at preflight or more than 8 MiB encoded output returns 413. Failed captures
never return a partial successful profile. Stats remain available after the active
capture finishes, even when profiles exceed budget.

These are admission/output limits, not a hard bound on Go's internal profile
working memory or execution time: allocation may continue after preflight, and
`runtime/pprof` cannot cancel its internal collection. Cancellation discards output;
the single capture stays busy until that work returns. HTTP timeouts bound socket
I/O, not runtime work. Even `ReadMemStats` briefly coordinates with the runtime;
collect sparingly (for example counters every 15 seconds, profiles minutes apart).

Normal return and handled SIGINT/SIGTERM close the listener and remove only its
own socket and run directory. The supplied directory and exported files remain.
A substituted file is preserved. SIGKILL/crash may leave a stale run directory;
the next run always uses a fresh one. Remove a stale directory only after verifying
its process is gone and that it contains no evidence to retain.
