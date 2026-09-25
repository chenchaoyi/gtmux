# Tasks

## One retrieval
- [x] `internal/knowledge/retrieve.go`: tokenizing, corpus statistics, scoring, one Search
- [x] `neighbours.go` keeps the public shape and calls it
- [x] `knowledgematch.go` becomes the echo's presentation layer only
- [x] no second tokenizer or second notion of a match anywhere in the package

## Ranking
- [x] idf weighting over the live base, smoothed so a two-entry base still matches (the
      plain form collapses to zero weights there; measured cost is one point of recall)
- [x] the floor moves to the weighted score, at a value the measurement supports
- [x] `neighbours` returns ten
- [x] test: a rare shared token outranks a common one

## The echo
- [x] ranks live entries through the shared retrieval
- [x] a Chinese goal returns hits (test with a real Chinese goal)
- [x] covers best-practices; accounts / corrections / environment stay out
- [x] test: the topics in and out of the echo are pinned

## The verb
- [x] `gtmux knowledge search "<text>"`, `--json`, `--topic`, `--kind`
- [x] usage both languages, helpdata entry, `docs/cli.md` + `.zh.md`

## The playbook
- [x] the consult section teaches `knowledge search`, playbook version 48

## Verify
- [x] re-measure with the REAL binary against the same pairs: recall@5 36% to 44%,
      recall@10 36% to 55% (the old one returned five, so ten was the same as five)
- [x] every new guard confirmed by re-introducing its defect
- [x] `make check` + `check-design.sh` by exit code; sync-specs + archive
