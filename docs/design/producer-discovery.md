# Codex discovery in complete producers

`radar.Collection` owns a lazy Codex path scope for one digest, usage request or
ordered sensor pass. Its `Agents` method samples rows again on every call. It is not
a fleet cache and must not be stored as a server-wide singleton.

Digest routes structured content through `Driver.ContentIn`; a disabled Content
capability stays disabled and custom/other drivers retain their readers. The built-in
Codex parser uses the existing incremental loader with operation-owned matching paths.
Usage uses `ForSessionScoped`, preserving the first/base rollout identity on which the
existing byte-offset counter depends. A continuation is not a new accounting file.

Path discovery is memoized; transcript data, token totals, status, identity verification
and consent are read afresh. The next collection enumerates newly created rollouts.
HQ watchdog/stuck/resolved sensors share only discovery, run in their original order
and gather rows again after previous sensors may have changed markers.

Direct transcript and usage APIs remain fresh per call. Limits' cached subscription
probe and daily-history catalogs retain their own policies; the one-discovery assertion
covers transcript path resolution, not every filesystem operation in a usage report.
No global cross-request cache, wire field or authorization change is introduced.

Fixture tests cover complete-producer discovery count, live content/state/usage changes,
permission revocation and base-file accounting. Mutation checks bypass digest or usage
scope sharing and try caching rows; each must fail the relevant regression test.
