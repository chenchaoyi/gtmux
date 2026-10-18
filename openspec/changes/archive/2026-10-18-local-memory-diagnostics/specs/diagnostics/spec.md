## ADDED Requirements

### Requirement: Memory observations are explicitly enabled and local only
A foreground serve SHALL enable memory observations only with `--memory-diagnostics DIR`. An empty default SHALL add no listener or periodic sampling. The caller SHALL supply an absolute existing owned 0700 directory, with a non-symlink leaf and protected ancestors. Each start SHALL create a fresh 0700 subdirectory and 0600 Unix socket. These routes SHALL NOT be registered on the phone/browser HTTP handler. Unsafe or unavailable paths SHALL disable diagnostics with a warning while allowing serve to continue. Service installation/removal flags SHALL reject the diagnostic option.

#### Scenario: Default or unsafe startup
- **WHEN** the flag is absent, or an enabled directory is unsafe or cannot host a Unix socket
- **THEN** no diagnostic listener is created and no permissive fallback is attempted
- **AND** a diagnostic startup error does not prevent the main service from starting

#### Scenario: Local capture and independent HTTP routes
- **WHEN** the owner connects to the private socket
- **THEN** GET /stats, /heap and /allocs are available there only
- **AND** those paths are unavailable on the application HTTP handler even to an authenticated owner

### Requirement: Memory captures preserve runtime settings and bound diagnostic work
Counters SHALL include PID, application/Go version, diagnostic start and capture times, allocator/heap/stack statistics, GC counts including forced GC, goroutine count and memory sampling rate. Profiles SHALL use standard Go sampled heap/allocs protobuf data and carry PID/version/time/GC metadata in response headers. Reads SHALL NOT force GC or change sampling/GC settings. Only one read SHALL execute at once; profiles SHALL have a five-second minimum admission interval, a 16384-record preflight limit and an 8 MiB encoded-output limit. Query parameters, request bodies and other methods SHALL be refused. Failures SHALL NOT return partial profiles as successful captures. Documentation SHALL distinguish these admission/output limits from Go's internal non-cancellable profile work and GC lag.

#### Scenario: Excessive or concurrent requests
- **WHEN** a profile exceeds an admission/output budget, or another capture is busy
- **THEN** it fails explicitly without queuing additional capture work or returning a partial successful profile
- **AND** subsequent counter reads remain usable after the active work ends

#### Scenario: Shutdown and stale paths
- **WHEN** serve returns normally or receives its handled stop signal
- **THEN** its diagnostic listener closes and only its own socket/run directory are removed
- **AND** a substituted file or caller-owned directory is preserved; an unclean exit cannot make a future start reuse a stale socket
