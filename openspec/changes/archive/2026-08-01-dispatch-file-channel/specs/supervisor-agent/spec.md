# supervisor-agent (delta)

## MODIFIED Requirements

### Requirement: HQ dispatches through the verified path, never a raw launch

The HQ playbook SHALL direct the supervisor to dispatch new work through
`gtmux spawn` (which applies the CONFIGURED proxy by construction and verifies
delivery), never a hand-rolled `send-keys` launch that would bypass the configured
proxy and 403 on a proxy-needing network. The `environment.md` knowledge seed SHALL
state that the configured proxy covers ONLY gtmux's own launch path, and that the
choice is explicit (`gtmux config agent-proxy` / `GTMUX_AGENT_PROXY`).

The playbook SHALL further fix the STANDARD ACTION for carrying a goal into that
dispatch: any goal that is not a short single line SHALL be written to a file and passed
as `gtmux spawn --goal-file <path>` (equivalently `gtmux send --message-file`), and the
playbook SHALL state the REASON — a goal passed as a command-line argument is necessarily
parsed by a shell, so a backtick, `$`, quote or newline inside it is executed or mangled
rather than delivered. The instruction SHALL be phrased as a mechanism, not as a caution:
this exact footgun was recorded in HQ's knowledge base twice and generalized into a rule
before it recurred, which is evidence that "remember to quote carefully" is not a
guarantee a supervisor can hold.

#### Scenario: Playbook points dispatch at `gtmux spawn`

- **WHEN** the HQ playbook and knowledge seeds are generated
- **THEN** they instruct dispatch via `gtmux spawn` and note that a bare
  `send-keys` launch is un-proxied and will 403

#### Scenario: Playbook fixes the file channel as the standard action

- **WHEN** the HQ playbook is generated
- **THEN** its dispatch section tells the supervisor to write any multi-line or
  special-character goal to a file and dispatch it with `--goal-file`, and states that a
  goal passed as an argument must survive shell parsing
