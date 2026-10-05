#!/usr/bin/env bash
# import-boundaries.sh [module-dir] — the import rules no compile error enforces
# (openspec/specs/code-architecture, "The knowledge base is one leaf package"):
#   - internal/mine never imports internal/knowledge;
#   - internal/knowledge imports nothing above the leaves: not app, hq, radar,
#     dispatchbridge or mine.
# Checked on the transitive imports, so going through another package breaks the rule
# too. check-design.sh runs this; it prints each violation and exits 1 on any.
#
# The spec required check-design.sh to enforce both since the knowledge extraction, but
# the gate only grepped internal/hq for the ledger's file names: a synthetic mine →
# knowledge import compiled and passed it (%12, 2026-10-06).
set -euo pipefail
cd "${1:-.}"
# A toolchain that cannot read the module (older than go.mod's go line, say) is "not
# checked", exit 2, never a violation: check-design.sh reads 1 as a broken boundary.
mod="$(go list -m)" || {
  echo "import-boundaries: go list -m failed; boundaries not checked" >&2
  exit 2
}
fail=0
check() { # check <package> <forbidden package>...
  local pkg="$1"; shift
  local deps
  deps="$(go list -deps -f '{{.ImportPath}}' "./internal/$pkg")" || {
    echo "import-boundaries: go list ./internal/$pkg failed" >&2
    exit 2
  }
  local f
  for f in "$@"; do
    if grep -qx "$mod/internal/$f" <<<"$deps"; then
      echo "internal/$pkg imports internal/$f, directly or through another package (code-architecture forbids it)"
      fail=1
    fi
  done
}
check mine knowledge
check knowledge app hq radar dispatchbridge mine
exit "$fail"
