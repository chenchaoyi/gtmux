## ADDED Requirements

### Requirement: Always-on service follows the selected backend

`gtmux tunnel --status` SHALL report Direct when the Direct LaunchAgent and
shared serve LaunchAgent are installed, including their loaded status and the
recorded URL. With no explicit `--backend`, `gtmux tunnel --service` SHALL
preserve an installed Direct backend; with an explicit backend it SHALL switch
as requested. A foreground tunnel command SHALL reuse a loaded always-on
tunnel of either backend.

#### Scenario: Direct already installed

- **WHEN** the user checks status and reruns `gtmux tunnel --service`
- **THEN** status names Direct and the command keeps the Direct route
- **AND** an explicit `--backend cloudflare` may still switch to Standard

### Requirement: Keep a successfully redeemed phone credential

After `POST /api/enroll` returns a device token for a v2 pairing code, the phone
SHALL save that Mac and token without requiring a second radar request. A
single-use code SHALL not be discarded because the initial radar request is
slow or the network drops after enrollment.

#### Scenario: Radar is slow after enrollment

- **WHEN** enrollment returns a device token but `/api/agents` is slow
- **THEN** the phone retains the issued token and can retry the connection
