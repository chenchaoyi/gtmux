# Design

A bounded archive reader validates the complete tar/gzip/age stream, with 128 MiB total
uncompressed data, 32 MiB per file and 5,000 entries and 50,000 knowledge records. Paths, links/special files and duplicate
names are refused before live writes. All staging is private. A restore prepares a complete
new directory first, moves the existing home aside and publishes by rename, rolling back
the old directory if publication fails.

Migration preview reads source files without changing the destination. Selection starts
empty. Staging has a versioned manifest and content digests, no board or credentials.
Knowledge review shows current live entries plus their original operation lineage; sensitive
families are excluded unless separately enabled. Collision with a different destination
history refuses that family. Identical history is skipped. Applied original operations
retain their IDs/metadata and acquire a migration source marker; a final withdrawal resets
old promotion/audience decisions. The target ledger changes by one atomic replacement;
a private pre-merge backup is kept. Source references still name the old Mac.

LOCAL.md is reviewed separately and never merged heuristically. Keeping current is the
default. Replacement uses atomic file publication and a private backup. Knowledge and
LOCAL application are separate decisions/receipts, so failure of one cannot hide the other.
Tools are attachments in staging only, stripped of executable permissions.

A running HQ must be stopped before live restore/apply, even if presently idle, to avoid
an old context writing over newly restored material. Preview and staging remain available.
The GUI calls the same CLI/core, gives plain exports an explicit acknowledgement, and
passes secrets on stdin only. Logs record IDs/counts, never passphrases or document bodies.

Startup, restore and apply share a stable nonblocking lock outside the HQ home; a
knowledge storage lock outside that home serializes ledger writes and complete exports.
Plain and encrypted exports use unique private temporary files.
