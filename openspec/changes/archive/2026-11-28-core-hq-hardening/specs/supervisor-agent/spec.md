## ADDED Requirements

### Requirement: HQ board bounds, fast snapshot receipts, frontend hygiene, and miner resilience

HQ SHALL truncate `Board()` on valid UTF-8 boundaries, warn when `board.md` exceeds 32 KiB or 250 lines, and cap playbook handoff logs to 10 entries. Snapshot listing SHALL validate `.json` receipts without re-decompressing existing archives. Hygiene and inspection SHALL accept valid `codex-frontends` socket symlinks and prune dead sockets and retired receipts, while `mine` SHALL lock, bound `emitted.json`, cache `last-pass`, and back off on errors.

#### Scenario: Oversized or multibyte HQ board is handled safely

- **WHEN** `board.md` exceeds 32 KiB or 250 lines or cuts across multibyte UTF-8 at the read cap
- **THEN** `Board()` returns valid UTF-8 and `doctor` and `self-check` surface the size warning
