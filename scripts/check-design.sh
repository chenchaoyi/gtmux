#!/usr/bin/env bash
# Design + architecture conformance check (run in CI).
#
# This is the automated half of the "did we actually follow the design?" /
# "is the architecture still sound?" review (see docs/test/TEST-PLAN.md). It can't
# judge visuals — that's manual acceptance against docs/design/mockup/ — but it
# guards the machine-checkable invariants so they can't silently drift.
set -euo pipefail
cd "$(dirname "$0")/.."

DESIGN=docs/design/DESIGN.md
THEME=macapp/Sources/GtmuxBar/Theme.swift
fail=0
note() { echo "design-check: $*"; }

# 1. Status palette: the app's colors MUST equal DESIGN §9's authoritative hex.
for hex in EF4444 06B6D4 22C55E 8E8E93; do
  grep -qi "$hex" "$DESIGN" || { note "#$hex not in $DESIGN (spec changed?)"; fail=1; }
  grep -qi "$hex" "$THEME"  || { note "status color #$hex from DESIGN missing in $THEME"; fail=1; }
done

# 2. Architecture: the menu-bar app stays native — no systray regression.
if grep -rqi "systray" macapp/Sources 2>/dev/null; then
  note "macapp must stay native AppKit (no systray dependency)"; fail=1
fi

# 3. Architecture: the app stays a CONSUMER — it must not re-implement agent
#    detection; it only reads `gtmux agents --json` and shells `gtmux`.
if grep -rqi "list-panes\|gatherAgents\|classifyAgent" macapp/Sources 2>/dev/null; then
  note "macapp must consume 'gtmux agents --json', not re-detect agents"; fail=1
fi

# 4. The CLI must stay cgo-free (only the Swift app is native). Skipped where
#    there's no Go toolchain (the macOS CI job is Swift-only; Linux CI enforces it).
if command -v go >/dev/null 2>&1; then
  if CGO_ENABLED=0 go build -o /dev/null ./cmd/gtmux 2>/dev/null; then :; else
    note "CLI failed to build cgo-free (CGO_ENABLED=0 ./cmd/gtmux)"; fail=1
  fi
fi

# 5. Spec validity: the OpenSpec capability specs must stay well-formed, so a
#    behaviour change can't land with a broken/missing spec. This is the automated
#    half of CLAUDE.md's "spec ⇄ code ⇄ test consistency" rule (the spec-matches-code
#    and archive-hygiene halves are a review-gate checklist item). Needs node; both
#    CI runners (ubuntu + macOS) have it preinstalled, so a missing npx = a real gap.
if command -v npx >/dev/null 2>&1; then
  if npx --yes @fission-ai/openspec validate --specs --strict >/tmp/openspec-check.log 2>&1; then :; else
    note "openspec spec validation FAILED (specs malformed / drifted):"; cat /tmp/openspec-check.log; fail=1
  fi
else
  note "npx not found — skipping openspec validation (install node to enforce it)"
fi

# 6. CLI command ↔ docs drift. Every user-facing command wired into the dispatch
#    (internal/app/app.go) MUST be listed in CLAUDE.md's command registry — this is the
#    exhaustive reference, so a new/renamed command that isn't there is a drift (exactly
#    how `attach` once shipped undocumented). Internal/plumbing commands are exempt via
#    HIDDEN. The curated `gtmux --help` usage + docs/cli.md are a deliberate SUBSET, so
#    we can't require completeness there — instead we check them in REVERSE (a doc must
#    not reference a command that no longer exists). Spec↔behavior + "is this command
#    worth documenting in the curated usage/cli.md" stay REVIEW-GATE items (see below).
APP=internal/app/app.go
HIDDEN=" tunnel-client save-tab-order options app "   # internal plumbing, not user commands
cmds=$(grep -oE 'case "[a-z][a-z0-9-]*"' "$APP" | sed 's/case "//;s/"$//' | sort -u)
# The command list is "Commands: `a`, `b`, … `uninstall-app`." — grab exactly that,
# truncating the prose that follows `uninstall-app` on the same line (else a command
# named in the description would falsely count as "listed").
listblock=$(awk '/Commands:/{f=1} f{print} /uninstall-app`\./{exit}' CLAUDE.md | sed -E 's/`uninstall-app`\..*/`uninstall-app`./')
for cmd in $cmds; do
  case "$HIDDEN" in *" $cmd "*) continue ;; esac
  needle=$(printf '`%s`' "$cmd")
  printf '%s\n' "$listblock" | grep -qF "$needle" || {
    note "CLI command '$cmd' (dispatched in $APP) is NOT in the CLAUDE.md command list — add it there (and weigh 'gtmux --help' usage + a docs/cli.md section)"; fail=1
  }
done
for cmd in $(grep -oE '^## `gtmux [a-z][a-z0-9-]*`' docs/cli.md | sed -E 's/^## `gtmux ([a-z0-9-]+)`/\1/'); do
  printf '%s\n' "$cmds" | grep -qxF "$cmd" || {
    note "docs/cli.md documents 'gtmux $cmd' but it is not a dispatched command (renamed/removed → stale doc)"; fail=1
  }
done

if [ "$fail" = 0 ]; then
  note "OK — status palette matches DESIGN §9; architecture invariants hold; specs valid; CLI commands documented"
else
  exit 1
fi
