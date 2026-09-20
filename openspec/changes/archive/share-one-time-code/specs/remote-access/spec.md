# remote-access (delta)

## ADDED Requirements

### Requirement: A share link can be handed over as a one-time code

A guest link SHALL be handable as a SHORT ONE-TIME CODE as well as a URL, so it can be
read out loud and typed on a surface that cannot paste. The owner mints a code for an
existing link; the guest opens the address with no secret in it and types the code; the
page redeems it for that link's token and keeps it, as the browser already does for an
owner pairing code.

A share code SHALL be bound to one link, single-use, and SHALL expire within ten minutes
of being minted. It SHALL be drawn from an alphabet a person can transcribe: Crockford
base32 without `I`, `L`, `O` and `U`, grouped for reading, matched without regard to case,
dashes or spaces. It SHALL carry at least 40 bits.

Redeeming SHALL create nothing: it returns the token the link already has, with the link's
own panes and expiry, so a code can never widen a scope. Revoking the link SHALL end every
code minted for it. A code that is not taken SHALL be refused with the same three reasons
a pairing code uses — expired, used, unknown — so a typo is legible.

The URL form SHALL remain exactly as it is: the code is another door to the same room, not
a replacement for the link.

#### Scenario: A browser that cannot paste

- **WHEN** the owner mints a code for a share link and reads it to someone at a TV browser
- **THEN** that person opens the bare address, types the code, and the page works as if
  they had opened the full link, with nothing secret left in the URL or in history

#### Scenario: The same code twice

- **WHEN** a code is redeemed and then entered again, by anyone
- **THEN** it is refused as used, and the first holder's access is unaffected

#### Scenario: The link is revoked

- **WHEN** a share link is revoked while a code minted for it is still within its ten
  minutes
- **THEN** redeeming that code is refused, since the token it would hand over is gone
