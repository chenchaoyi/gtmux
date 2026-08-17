# Tasks — language-loose-ends

Status: IMPLEMENTED (2026-08-17).

- [x] 1. hqLocalTemplate: en/zh editions selected by i18n.Lang; seed-once
     unchanged (an existing LOCAL.md is never rewritten). Tests: template
     follows SetLang; a language switch does not touch an existing file.
- [x] 2. seedResult.LangSwitch + a dedicated notice for a same-version
     language switch. Test: the notice names the language, not "v32 → v32".
- [x] 3. Spec delta: supervisor-agent language requirement gains the LOCAL.md
     clause.
- [x] 4. Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
     `scripts/check-design.sh`, `openspec validate --strict`.
