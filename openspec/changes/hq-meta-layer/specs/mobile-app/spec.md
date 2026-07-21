# mobile-app — delta

## ADDED Requirements

### Requirement: Mobile HQ card shows an intelligence headline, not fleet pips

The mobile HQ (chief-of-staff) card SHALL NOT render a row of per-worker "fleet pips"
(they duplicate the section list below it). Its subtitle SHALL be the same deterministic
intelligence headline as the menu-bar card: it names the worker that needs the user plus
a count of the rest when something is waiting, or reads as "all normal" when quiet,
coloured for attention when a worker or HQ itself needs the user.

#### Scenario: A worker is waiting

- **WHEN** the fleet has one or more waiting workers
- **THEN** the mobile HQ card subtitle names the first waiter and how many others are normal, with attention colour, and renders no pip row

#### Scenario: All quiet

- **WHEN** no worker is waiting
- **THEN** the mobile HQ card subtitle reads as "all normal", dim, with no pip row
