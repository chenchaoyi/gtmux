# tasks — hq-knowledge-on-phone

## 1. Domain (internal/hq)

- [x] 1.1 Extract the exported verbs `KnowledgeLand(id, ref)` / `KnowledgeRetire(id, why)`
      from the CLI verbs, so the CLI parses arguments over the same function serve calls.
- [x] 1.2 Add `KnowledgeIndex()` — live entries without bodies, topic counts, promotion
      and candidate summaries — and `KnowledgeEntry(id)`.
- [x] 1.3 Tests: the extracted verbs behave identically through both callers; the index
      shape holds on an empty base.

## 2. Serve (internal/server, internal/app)

- [x] 2.1 Handlers for the three endpoints, guest-refused.
- [x] 2.2 Wire the deps in `serve.go`; a nil dep serves empty rather than 503.
- [x] 2.3 Tests: guest 403, empty base 200, land/retire round trip, malformed act 400.

## 3. Mobile

- [x] 3.1 Client methods + types.
- [x] 3.2 `knowledgeModel.ts` — what leads (the promotion queue), the newest-entries
      slice, topic grouping, staleness of a pending promotion. Tested as rules.
- [x] 3.3 `KnowledgeSheet.tsx` — index → entry, the two actions, empty states.
- [x] 3.4 The header disclosure's second document row.
- [x] 3.5 Tests: model rules + view render.

## 4. Docs

- [x] 4.1 `api/contract.md` — the three endpoints.
- [x] 4.2 `docs/design/MOBILE.md` §17 — the surface and what is deliberately not on it.
- [x] 4.3 `docs/cli.md` — note the second door on the knowledge section.
