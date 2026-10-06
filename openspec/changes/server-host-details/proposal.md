# server-host-details

## Why

The user (2026-10-06): on the phone's Servers page they want to see what each paired Mac
actually is — its real host name, its OS version, and whatever else gtmux can tell — either
on the list or on a details page. A Mac renamed on the phone, or one of several similar
Macs, could only be told apart by its address. Nothing on the wire said what the machine is:
`/api/health` is deliberately anonymous and `/api/theme` / `/api/usage` answer other
questions.

## What Changes

- `gtmux serve` gains `GET /api/host` (owner only; a guest gets 403): host name, macOS
  Computer Name, OS name / version / build, architecture, CPU model, core count, memory,
  boot time, tmux version, gtmux version and when serve started. Read once and cached
  (`internal/hostinfo`), each probe a time-limited command.
- The phone asks it for each owned Mac that answers its reachability probe (kept five
  minutes) and adds "· Studio · macOS 26.1" to that row's existing status line, never a
  third line, so a late answer does not move the list.
- ••• on a server row gains **Details**: a sheet with what the phone keeps (name, address,
  access) and what the Mac reported (names, system, chip, cores, memory, uptime, gtmux,
  serve uptime, tmux). An older gtmux (404), a share link (never asked) and an unreachable
  Mac each say why the Mac's part is missing.

## Surfaces

- 终端 (terminal, incl. remote attach): not applicable — the CLI runs ON the machine; `gtmux
  doctor` / `gtmux status` already describe the local install.
- 菜单栏 (menubar): not applicable — the menu-bar app lives on the Mac it would describe.
- 手机 (phone): the subject of this change (Servers page status clause + Details sheet).
- iPad: the same Servers page and sheet (ContentColumn; one implementation).
- Web: deferred — the browser mirror has no server list (one Mac per page); the endpoint is
  there if it grows one.

## What does NOT change

- `/api/health` stays unauthenticated and anonymous.
- Guests see nothing new; the share-link capability does not cover the machine.
- The two-line Servers row and its probe cadence.
