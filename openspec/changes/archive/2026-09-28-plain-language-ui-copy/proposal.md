# Plain language for access and knowledge actions

## Why

The Mac's three Anywhere entry points use different confirmations, including
"expose this Mac to the whole internet" and an unexplained `token`. Pairing,
sharing, diagnostics and knowledge screens contain similar literal translations,
unnecessary handoff instructions, and misleading security claims. The same
knowledge act also has different Chinese names on Mac and phone.

## What changes

Use one factual Anywhere confirmation at every Mac entry point. State who can
connect and that the setting survives a restart until turned off. Rewrite the
affected explanatory and error text around the action a user can take. Use
"不再适用" for the knowledge `retire` action on both apps, without changing its
ledger behavior. Keep English and Chinese copy aligned.

## Surfaces

- **terminal**: no CLI behavior change.
- **menubar**: remote access, pairing, sharing, diagnostics, server mode and
  knowledge copy.
- **phone**: pairing and diagnostics messages, knowledge actions and help.
- **iPad**: the same mobile copy as phone.
- **web**: no copy change.
