# Source inheritance on KB revisions

[中文](kb-source-lineage.zh.md)

Both ordinary supersede and review apply keep omitted `seq_range`, `seqs`,
`capture`, `pane`, `task`, `legacy` and `migration` from the live predecessor.
New operations persist these fields inside the existing ledger lock. Replay uses
the same absent-only rule; it does not rewrite ledger history or search retired
revisions. Explicit nonzero successor fields win; a legacy origin remains true.
Inherited sequence slices have independent storage. Existing structured Sources
continue to merge by candidate ID without replacing their payloads.

`seq` is the new mutation event watermark; `candidate_result` is a new operation's
receipt and is not inherited. Review evidence/reason/time/operation ID remain new.
Existing provenance, tags and current hit history still fold normally. Review
continues to enforce digest/CAS, atomic append and idempotent retries. Dry runs
remain read-only. Ordinary supersede still rejudges distribution; reviewed revise
keeps audience/status/protection and reopens landing. Inherited action cards become
tentative. Ordinary supersede resets its alternate; review preserves omitted/null
alternates exactly and validates an explicit replacement in the same transaction.
New review preparation explicitly stores the retained alternate. Replay uses that
recorded value; historical omitted alternates keep their original reset semantics,
respecting explicit ledger clears and ordinary resets. Retained text is not proof of translation equivalence;
see the [review contract](../guides/knowledge-review.md).

Tests use synthetic disposable HOME fixtures and actual CLI/apply/read/render
paths. No production KB proposals or private conversation bodies are fixtures.
The reviewed test design is [KBL-001–004](../test/designs/kb-source-lineage-20261113.json).
