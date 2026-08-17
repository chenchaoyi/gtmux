# Tasks — machine-level-language

Status: IMPLEMENTED (2026-08-17).

- [x] 1. resolveLang: GTMUX_LANG > config lang > locale > en; config "auto"
     falls through to the locale; broken explicit values resolve to en.
     Table test (11 cases).
- [x] 2. `gtmux config lang [en|zh|auto]` + usage text (en+zh).
- [x] 3. docs/cli.md: language line + `### lang` section.
- [x] 4. Spec delta: supervisor-agent language requirement's resolution order
     gains the config layer and the one-machine-one-language promise.
- [x] 5. Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
     `scripts/check-design.sh`, `openspec validate --strict`.
