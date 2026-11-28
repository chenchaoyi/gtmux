## 1. OpenSpec & Sealed Test Design

- [x] 1.1 Author OpenSpec proposal, design, and delta specs for `polish-agy-ux-and-composer-region` and validate with `openspec validate --strict`.
- [x] 1.2 Record and seal test design `docs/test/designs/agy-ux-polish-20261127.json` in a dedicated commit before editing test or production files.

## 2. Core Implementation & Bilingual Documentation

- [x] 2.1 Update `internal/dispatch/region.go` (`SplitInputRegion`) to select an adjacent upper prompt-bearing composer box when the bottom-most box is a non-prompt auxiliary drawer, with unit tests in `internal/dispatch/region_test.go`.
- [x] 2.2 Add cgo-free `.pbtxt` title parsing (`AgySessionTitle` and `SessionTitles("agy")`) in `internal/transcript/` and wire `Antigravity` placeholder title stripping + bound title lookup in `internal/radar/`, with unit tests in `internal/transcript/` and `internal/radar/`.
- [x] 2.3 Fix pre-detach `originLifecycle` classification and `rootConversationId` subagent filtering in `internal/hook/`, guard `"modelName": "auto"` and `AGY_HOME` scoping in `internal/agentprofile/`, filter standalone `<SYSTEM_MESSAGE>` inputs and normalize `search_web` / `agy` tools in `internal/transcript/`, and update bilingual `docs/design/agent-profiles*.md`.

## 3. Verification & Archive

- [x] 3.1 Map test selectors in `docs/test/implementations.json`, refresh test designs, archive `polish-agy-ux-and-composer-region`, and pass `make check`, `./scripts/check-design.sh`, and `test_design.py --pr-history`.
