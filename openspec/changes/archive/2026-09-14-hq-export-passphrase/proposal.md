# Change: hq-export-passphrase

> STATUS: implemented 2026-09-14 (same PR). Proposed 2026-09-14, on the commander's direction 「先只做 2 吧，要设计一下用户体验，用户在备份过程中能够体验较好地指定口令，并且做好确认」. Implemented in the same PR.

## Why

HQ's memory export — the board, the knowledge base, `LOCAL.md` — is the copy that leaves the
machine: a USB stick, a synced folder, AirDrop to another Mac. The commander keeps personal
and account detail in the knowledge base on purpose (「为了方便，而且电脑是自己的」), and the
export carried it as plain text. Inside the Mac, FileVault covers the disk; the export was the
one copy nothing covered.

Of the ways to add protection, this is the cheap and certain one: the file that travels gets a
lock. The knowledge base itself is not encrypted and no lint nags about what it holds — the
commander asked for neither, and the base is deliberately distributed into the agents'
instruction files and HQ's context, where at-rest encryption would protect nothing.

## What Changes

1. **`gtmux hq --export` locks the file with a passphrase.** The archive is written as an
   age file (`.tar.gz.age`, a standard format any age tool opens, so the case the export
   exists for — gtmux may not be there to read it back — still holds). The passphrase comes
   from `--passphrase-stdin` (an app), `GTMUX_HQ_PASSPHRASE` (a script), or a terminal
   prompt typed twice, unechoed; eight characters at least; a mismatch asks again. `--plain`
   keeps the unlocked form.
2. **`gtmux hq --import` recognises a locked export**, asks for the passphrase the same
   ways, and refuses a wrong one before anything moves: the existing memory stays where it
   is.
3. **The menu bar's Export… is a sheet**: the passphrase first (two fields, a show toggle, one
   hint line that says the one thing to fix), the place second (the save panel), then a
   confirmation page with the path, the size, and how to open it. The passphrase can be kept
   in this Mac's keychain, so the next export is one click; a remembered one shows as a single
   line with "Change…". It reaches the CLI over stdin, never argv.
4. **The memory line says when the last export was and whether it was locked**, in the CLI
   (`gtmux hq --memory`) and the reader window — the one off-machine copy gtmux can vouch for.

## Surfaces

- 终端 (terminal, incl. remote attach): `--export` / `--import` prompt for the passphrase;
  `--plain`, `--passphrase-stdin` and `GTMUX_HQ_PASSPHRASE` for scripts.
- 菜单栏 (menubar): the export sheet, keychain memory, the confirmation page, the memory line.
- 手机 (phone): not applicable — the phone keeps no export and never writes one.
- iPad: not applicable, same as the phone.
- Web: not applicable — the shared page is a read-only mirror.

## What does NOT change

- The knowledge base, the board and the daily snapshots: untouched and unencrypted on disk.
- What an export contains, and the import's move-aside rule.
- No lint, no warning about what the knowledge base holds.
