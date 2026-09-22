#!/usr/bin/env bash
# vulncheck.sh — fail when the code REACHES a known vulnerability that already has a fix.
#
# Why this exists: CI checked style, types, tests and design, and never asked whether a
# dependency was known to be vulnerable. A 2026-09-22 self-check found nine reachable
# advisories that way, by hand: a chisel ACL bypass, two x/crypto/ssh denials of
# service, and standard-library fixes; the release toolchain (Go 1.25) had also left
# support, so it would stop receiving them at all.
#
# The rule is "a fix exists and we have not taken it". A reachable advisory with NO fix
# yet is printed and does not fail: a gate that stays red until someone else ships a
# patch is a gate everyone learns to ignore. Unreachable advisories (in a module we
# import but never call into) are not reported, which is govulncheck's own default.
#
# The standard library is judged against the Go that runs this. CI installs the newest
# patch of its pinned minor, so a stdlib finding there means a security release exists
# that CI has not picked up; locally it usually means your toolchain is behind.
set -euo pipefail
cd "$(dirname "$0")/.."

# PINNED, not @latest, for the same reason staticcheck is (see the Makefile).
GOVULNCHECK=golang.org/x/vuln/cmd/govulncheck@v1.8.0

out="$(mktemp)"
trap 'rm -f "$out"' EXIT
# govulncheck exits non-zero when it finds anything; the verdict is ours to make below.
go run "$GOVULNCHECK" -format json ./... >"$out" || true

python3 - "$out" <<'PY'
import json, sys

buf = open(sys.argv[1]).read()
dec, i, osv, reach = json.JSONDecoder(), 0, {}, {}
while i < len(buf):
    while i < len(buf) and buf[i].isspace():
        i += 1
    if i >= len(buf):
        break
    obj, i = dec.raw_decode(buf, i)
    if "osv" in obj:
        osv[obj["osv"]["id"]] = obj["osv"]
    f = obj.get("finding")
    # A finding is REACHABLE when its trace starts at a function in this module.
    # The trace runs from the vulnerable symbol (first) to the entry point in our code (last).
    if f and (f.get("trace") or [{}])[0].get("function"):
        vuln, entry = f["trace"][0], f["trace"][-1]
        reach.setdefault(f["osv"], (vuln.get("module") or "?", f.get("fixed_version") or "", entry))

if not reach:
    print("vulncheck: OK — no reachable known vulnerability")
    sys.exit(0)

fixable = {k: v for k, v in reach.items() if v[1]}
for k, (mod, fix, frame) in sorted(reach.items()):
    where = "%s:%s" % (frame.get("position", {}).get("filename", "?"), frame.get("position", {}).get("line", "?"))
    if frame.get("function"):
        where += " (%s)" % frame["function"]
    what = osv.get(k, {}).get("summary", "")
    verdict = ("FIX AVAILABLE in %s" % fix) if fix else "no fix upstream yet (warning only)"
    print("vulncheck: %s  %s  %s\n    %s\n    reached from %s" % (k, mod, verdict, what, where))
if fixable:
    print("vulncheck: FAIL — %d reachable vulnerabilit%s with a fix available; upgrade the module "
          "(go get <module>@<fixed>) or the Go toolchain for the standard library"
          % (len(fixable), "y" if len(fixable) == 1 else "ies"))
    sys.exit(1)
print("vulncheck: OK — only advisories with no fix upstream yet, listed above")
PY
