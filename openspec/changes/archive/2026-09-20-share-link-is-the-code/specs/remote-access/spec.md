# remote-access (delta)

## REMOVED Requirements

### Requirement: A share link can be handed over as a one-time code

**Reason**: The link and its code were two artifacts with identical access behind them,
and the only difference was how long the hand-off stayed valid. They are one now.
**Migration**: A link's code is minted with the link and read off `GET /api/share/link`;
`gtmux share code` is retired. Links already handed out keep their `#g=<token>` form.

## ADDED Requirements

### Requirement: A share link is its own short code

A guest link SHALL carry a CODE beside its token, minted with the link and lasting exactly
as long as it does. The code is the link's public form: the address with `#code=<code>`,
which is clickable and pasteable, or the address and the code said as two lines, which is
readable out loud. Both open the same access; there is one artifact, not two.

Redeeming a code SHALL return the link's own token, creating nothing, so a code can never
widen a scope. It SHALL be reusable and SHALL NOT expire on its own: revoking the link or
reaching its expiry ends the code with everything else about that link.

A code SHALL be at least 40 bits, drawn from an alphabet a person can transcribe
(Crockford base32 without `I`, `L`, `O` and `U`), grouped for reading, and matched without
regard to case, dashes or spaces. Its length and the bound on guessing are ONE decision:
failed redeems SHALL be bounded per caller and in total, and the length SHALL keep the
tries available in a YEAR far below the combinations, since a lasting code has no window
that closes for it.

The code SHALL live in the URL's FRAGMENT rather than its query, so that it is not sent to
the tunnel, the proxy in front of it, or their logs. The raw-token form (`#g=<token>`)
SHALL keep working for links already handed out.

#### Scenario: One thing to hand over

- **WHEN** the owner mints a share link
- **THEN** they get one address carrying `#code=`, plus the two lines to read out if the
  other end cannot paste, and both open the same access

#### Scenario: A browser that cannot paste

- **WHEN** someone opens the bare address and types the code
- **THEN** the page holds the link's credential from then on, exactly as if they had
  opened the link, and typing it again later works as well

#### Scenario: A terminal guest

- **WHEN** someone runs `gtmux attach <link>` or `gtmux attach <host> --code <code>`
- **THEN** it connects with the link's scope and keeps the token for that host, so a later
  attach needs neither

#### Scenario: The link is revoked

- **WHEN** a share link is revoked
- **THEN** its code stops being accepted, and every device that redeemed it loses access
