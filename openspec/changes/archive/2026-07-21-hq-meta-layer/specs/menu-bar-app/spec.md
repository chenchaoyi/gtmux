# menu-bar-app — delta

## ADDED Requirements

### Requirement: HQ card shows an intelligence headline, not fleet pips

The menu-bar HQ (chief-of-staff) card SHALL NOT render a row of per-worker "fleet pips"
(they duplicate the section list and the summary count, and are anonymous). Its subtitle
SHALL be a deterministic intelligence headline synthesized from the worker fleet: when a
worker is waiting, it names the one that needs the user plus a count of the rest that are
normal; when nothing is waiting, it reads as "all normal, nothing needs you". The
headline is coloured for attention (red/amber) when a worker or HQ itself needs the user,
and dim when quiet.

#### Scenario: A worker is waiting

- **WHEN** the fleet has one or more waiting workers
- **THEN** the HQ card subtitle names the first waiter and how many others are normal (e.g. "api needs you · 4 others normal"), with attention colour — and shows no pip row

#### Scenario: All quiet

- **WHEN** no worker is waiting
- **THEN** the HQ card subtitle reads as "all normal — nothing needs you", dim, with no pip row
