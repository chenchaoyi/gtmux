# Tasks — hq-promote-anywhere

Status: IMPLEMENTED (2026-08-17). Text-first change; the one code seam is the brief's closing
render, tested both ways (target present / absent).

- [x] 1. Brief render: the closing instruction names the promotion's `--target`
     when present; otherwise a neutral carrier list (project AGENTS.md/CLAUDE.md ·
     team runbook · LOCAL.md · gtmux repo or a GitHub issue with this brief).
     Tests: both renders; the `land` instruction always present.
- [x] 2. Playbook v27: the charter-level definition broadens to "belongs in a
     durable rule carrier" with gtmux's repo as ONE carrier; the corrections and
     distill teaching drop the repo-only landing; `land --ref` examples include
     an issue URL and a runbook name. `hqPlaybookVersion` 26 → 27.
- [x] 3. Doctor flagged-note copy: "carry it into the gtmux repo" → carrier-
     neutral. `docs/cli.md` promotion paragraph likewise.
- [x] 4. Spec deltas: hq-knowledge promotion requirement MODIFIED (carrier
     definition + brief closing); supervisor-agent correction-loop MODIFIED
     (charter-level definition + landing path).
- [x] 5. Gates green: `make check`, `CGO_ENABLED=0 go build ./cmd/gtmux`,
     `scripts/check-design.sh`, `openspec validate --strict`.
