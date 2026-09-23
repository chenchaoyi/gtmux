#!/usr/bin/env bash
# verify-download.sh <file> <expected sha256> — say whether this file is the pinned one.
#
# It exists as its own script so the guard can be TESTED at command level: a source that
# serves the wrong bytes has to be refused, and the only way to know that is to run this
# against a file that does not match and watch it fail. install-server.sh calls it before
# it installs anything it downloaded, which is what makes a download mirror safe to use:
# the trust is in the checksum, not in whoever served the file.
set -euo pipefail
file="${1:?usage: verify-download.sh <file> <sha256>}"
want="${2:?usage: verify-download.sh <file> <sha256>}"

[ -f "$file" ] || { echo "verify: $file is not there"; exit 1; }
if command -v sha256sum >/dev/null; then
  got="$(sha256sum "$file" | cut -d' ' -f1)"
elif command -v shasum >/dev/null; then
  got="$(shasum -a 256 "$file" | cut -d' ' -f1)"
else
  echo "verify: no sha256sum or shasum on this box; refusing to install unverified bytes"
  exit 1
fi
[ "$got" = "$want" ] || { echo "verify: checksum mismatch (got $got, want $want)"; exit 1; }
echo "verify: checksum matches"
