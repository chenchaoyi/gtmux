# Design

Codex has chat records interspersed with potentially huge screenshots, tool
results and compaction snapshots. A raw byte tail does not bound useful history.
Use a record reader with a 64 KiB buffer and structural JSON header inspection.
Known non-chat records are drained; incomplete/reordered headers fall back to
normal parsing. Relevant records can exceed the buffer and remain intact.

The cache stores the next complete-line offset separately from file size. A
partial final JSON record remains pending until its newline arrives. Growth
continues the last retained turn using cloned segment and tool slices, preserving
snapshots already returned to other callers. Shrink, replacement and same-size
rewrite invalidate the cache. Read errors do not commit partial cache updates.

Streaming retention uses the existing 800-turn cap. HTTP response budgeting and
mobile display windows are unchanged. This addresses raw-log truncation without
claiming unlimited access to all older turns or recovering overwritten terminal
frames. No app binary change is required; the server reader must be updated.
