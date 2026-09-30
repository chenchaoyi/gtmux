# Moving HQ to another Mac

## Restore or move

The menu bar offers **HQ → Knowledge → Import…** beside Export:

- **Restore HQ backup** restores the complete archive, including the old board and
  built-in instructions. It retains the current HQ home as a separate backup.
- **Move from another Mac** carries selected long-term content into the destination HQ.
- **Continue staged migration** reopens a review after closing the window.

These flows are implemented. `gtmux hq --import` remains full restore; it is not selective
migration. The Mac's encrypted export and the phone/iPad's plain `.tar.gz` copy both work
as sources. A plain copy needs explicit acknowledgement in the UI or `--allow-plain` for
CLI staging. Passphrases are read locally and passed to the CLI on stdin, never argv.

Archive selection and preview use compact sheets. Select a backup first; an encrypted
backup then asks for its password. The form checks the file header, not its extension;
full validation still happens in the CLI. Each step keeps Cancel/Back and its primary
action together at the bottom. The knowledge and personal-requirements review expands
into a bounded list/detail or side-by-side workspace. Busy operations prevent dismissal.

## What can move

| Content | Behavior |
|---|---|
| Knowledge ledger | Optional; current live entries and the history/source snapshots needed for their revision lineage. Review before application. |
| `LOCAL.md` personal requirements | Optional; compare beside current text. Keep current by default; replacement requires a separate decision. |
| `knowledge/tools/` attachments | Optional; private staging only, with execute permissions removed. Never installed or run automatically. |
| Board, sessions, pending leads, unrelated retired entries, other notes, built-in instructions and generated distribution files | Excluded from migration. Rebuild from the new environment. |
| Connection credentials, pairing tokens, tunnel accounts and permission state | Not migrated. Authorize the new Mac separately. |

All choices start unchecked. A knowledge family with any historical sensitive mark is
excluded unless sensitive content is explicitly selected. Migration does not infer that
an entry is portable from its topic, kind or old audience. Review paths, environment
claims and authority limits yourself before accepting an entry.

## Review and application

1. Choose an archive and enter its passphrase when encrypted. Preview validates it and
   shows knowledge counts, sensitive count, personal requirements availability and tools.
   Preview does not write to HQ. It does not claim a creation date absent from the source.
2. Select content and stage it. The versioned manifest, content digests and selected files
   live privately under the gtmux state directory's `hq-migrations/<ID>/`.
3. Inspect knowledge in the right detail pane; select entries and confirm their
   applicability to this Mac. Different histories sharing an ID are conflicts and cannot
   be imported. Identical complete source history is skipped, including history already
   retired or revised locally; it never resurrects an old entry. Partial overlapping
   histories are conservatively refused, not auto-merged.
4. Exit HQ before applying or restoring, even if it is idle. A live agent retains old
   context and could overwrite restored content. No reset or pane input is sent. HQ
   startup, restore and application share a lock and report contention without waiting.
5. Apply selected knowledge as one atomic ledger replacement with a private pre-merge
   backup. Original IDs, operation history, language halves and source snapshots survive.
   Imported operations carry a source archive digest. A final withdrawal resets old
   audience, promotion and landing decisions: the new entries remain within HQ until a
   new distribution decision. Existing topic definitions are reused, not overwritten.
6. Apply personal requirements separately if desired. Compare both full texts, explicitly
   choose replacement, and retain a private backup. A digest guard rejects a changed
   destination until it is reviewed again. Identical text is skipped. Knowledge and
   personal requirements have separate application results, not one shared transaction.

Receipts report operation/stage IDs, imported/skipped counts, backup location and whether
publication committed. Failures before publication leave the destination unchanged. A
view or sync failure after knowledge commit is reported as committed; repair with
`gtmux knowledge render`, rather than importing again. Diagnostic/event logs contain
IDs and counts, not passphrases, knowledge bodies or personal requirement text. Source
session, event sequence and path metadata still refer to the **old Mac**, not local events.

## Validation and limits

The entire tar/gzip/age stream is validated before the current home can be moved. Limits
are 128 MiB total archive/decompressed data, 32 MiB per file, 5,000 archive entries and
50,000 knowledge records; age scrypt work factor is capped at 18. Unsafe paths, duplicate
normalized paths, links/special files, bad checksums, unsupported ledger operations,
unknown fields and invalid lineage fail explicitly. Staged digests and paths are checked
again on review/application. Staging directories are private and files are non-executable.

Legacy Markdown-only archives can be fully restored, or personal requirements/tools can
be staged, but knowledge needs manual curation into a ledger before selective migration.
No facts or provenance are invented. Conflicting IDs are left for explicit knowledge
ledger revision, never overwritten just to finish a move. Staged attachments and backups
are retained; this feature does not delete them automatically.

## Surfaces

The Mac menu bar owns file selection and guided review; `gtmux hq migrate --help` exposes
the same local core for CLI users. Phone and iPad keep full copy/export without remote
import. Web gains no archive authority or import endpoint. No account, server or external
memory service is required.
