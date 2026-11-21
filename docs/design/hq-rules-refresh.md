# Refresh HQ rules without controlling sessions

[中文](hq-rules-refresh.zh.md)

`gtmux hq --refresh-rules` is a standalone maintenance action for an existing
managed AGENTS.md. It returns before tmux discovery, resource sampling, seeding,
enrollment, input, focus and launch. Other HQ options cannot be combined with it;
help remains read-only. No tmux installation or running HQ is required.

Use the same existing playbook version/language decision and `playbookIn` renderer.
Keep the installed language. Equal or newer entries are unchanged. An exact current entry can restore missing bundled runbooks. Require
an owned regular writable file with the exact first-line managed marker and a known
language. Missing, legacy/unmanaged, link and special files are refused rather than
migrated. Bound reads to 1 MiB and use the existing private no-follow reader.

Hold the existing nonblocking startup/import lock. Back up the exact old bytes with
exclusive bounded collision handling, never pruning earlier backups. Reuse the
existing atomic private writer for AGENTS.md; do not seed LOCAL/CLAUDE/notes/knowledge.
Check the observed original identity/content again before publication. Cooperating
writers share the lock, but external editors do not: this is not an external atomic
CAS. Finish manual editing before refresh. Pre-publication failures preserve the old
file; post-rename directory-sync errors may leave the complete new file and backup,
and remain errors. No in-place partial-write or automatic rollback claim.

Publish the matching versioned runbooks before the new entry; preserve old
editions and refuse to replace a conflicting runbook. Writes belong to the
managed entry, runbooks, backup, lock and necessary parent directory handling.
The command leaves other files, session markers, inputs and services alone.

Tests and chronology: `docs/test/designs/hq-rules-refresh-20261113.json`. Self-review
precedes implementation; independent source review and normal shipping are required
before using the installed entry to adopt the shipped charter in the real HQ home.

Boundary follow-up: recognize the standalone token before consuming option values. Verify backups through their created descriptor and a bounded no-follow identity recheck; failed/changed paths are retained, never blindly removed. Mixed user/knowledge blocks and unknown trailing instructions are refused without migration. A dedicated single-link private lock open rejects hardlinks before chmod. Existing external-editor publication checks remain best-effort.

Known user/knowledge block marker prefixes reserve independent instructions wherever they occur, including inline begin/end positions accepted by the existing knowledge protocol. Refuse refresh without changing the effective file; backups do not substitute for active instructions. This does not merge arbitrary unmarked text.
