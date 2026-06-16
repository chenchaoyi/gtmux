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

# 4. The CLI must stay cgo-free (only the Swift app is native).
if CGO_ENABLED=0 go build -o /dev/null ./cmd/gtmux 2>/dev/null; then :; else
  note "CLI failed to build cgo-free (CGO_ENABLED=0 ./cmd/gtmux)"; fail=1
fi

if [ "$fail" = 0 ]; then
  note "OK — status palette matches DESIGN §9; architecture invariants hold"
else
  exit 1
fi
