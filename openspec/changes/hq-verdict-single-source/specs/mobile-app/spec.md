# mobile-app (delta)

## ADDED Requirements

### Requirement: The HQ page headline renders the core's verdict

The HQ page's assessment headline SHALL render the verdict served with the digest rather
than deriving one from the rows it happens to display. Deriving it locally is what let the
phone report a quiet fleet while the same moment's menu bar reported a machine under
pressure — the phone's local rule considered neither the supervisor's own waiting state
nor the machine's resource tier.

The sentence SHALL be composed on the device, in the reader's own language, from the
served state and facts. When no verdict is served the page SHALL fall back to its local
derivation rather than showing nothing.

#### Scenario: The phone speaks on a state it used to miss

- **WHEN** the served verdict is the supervisor's own call, or the machine's resource
  tier, while every worker is idle
- **THEN** the HQ page headline reports it, in the reader's language, instead of "all
  normal — nothing needs you"
