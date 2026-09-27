# Knowledge lint triage (2026-09-28)

A private local HQ report records each of the original 27 similar pairs, both entry IDs, ledger provenance, and the reason for keeping them separate. It also lists provenance fields for the unlinked entries. Private entry bodies and machine account details were not committed to this public repository.

| Finding | Evidence | Outcome |
| --- | --- | --- |
| 27 `near-duplicate` pairs | Compared each pair's text, scope, and provenance. None was a confirmed synonymous duplicate; 22 pairs already linked directly. The others cover different failure judgments, an individual tool versus its workflow, or distinct lessons from one incident. | Added real links for two parent/child pairs; left three unrelated pairs unlinked. Text changes leave 25 similarity candidates, labeled `review` rather than 25 defects. |
| 265 `orphan` entries | No `[[link]]` only means a standalone graph node. Topic, full-text search, and neighbours still index it; a link is not required. | Two title-only entries gained sourced bodies and verification limits; one script guide now states its source boundary; three existing references became actual links. The remaining 262 have bodies, provenance, and ledger sequence numbers and are `info` standalone entries. A ledger sequence alone does not prove a historical fact. |
| Alternate-language blind spot | The old linter scanned only the primary-language body, missing wiki references in the other language. | Both bodies now contribute links and graph edges. An English `[[links]]` example was excluded as a placeholder, and one historical reference to a superseded entry was made explicit. Broken, ambiguous, and outdated links are all zero. |

The checker keeps the `orphan` and `near-duplicate` JSON check names for compatibility and adds `severity`: `info`, `review`, or `issue`. Terminal output and the HQ self-check summary distinguish these levels. No entry was deleted, no distinct facts were merged, and no artificial links were added to reduce a count.

Verification: `go test ./internal/knowledge`; the new `knowledge lint --json` on 732 live local entries reports `near-duplicate=25`, `orphan=262`, `broken-link=0`, `ambiguous-link=0`, `outdated-link=0`, and `unmarked-sensitive=0`. Physical-device screen reader and phone/iPad layout acceptance remain open. This round did not build a native app or cut a release.
