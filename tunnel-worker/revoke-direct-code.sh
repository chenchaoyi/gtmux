#!/usr/bin/env bash
# revoke-direct-code.sh <code> — revoke a Direct code AND every device account it minted.
#
# Deleting the code alone used to be all revocation was, and it revoked nothing: every
# buyer held the one shared chisel secret. Accounts are per device now, and they live in
# the registry (src/direct.ts), so revoking removes them there first; the Direct server
# drops them at its next authfile sync (every 10s) and those devices stop connecting.
#
# Order matters: the accounts go first, the code second. If the first step fails, the
# code is still there and the whole thing can simply be run again.
#
#   ./revoke-direct-code.sh gtd-<24hex>
#   KV_TARGET=--local ./revoke-direct-code.sh <code>   # rehearse against wrangler's local KV
set -euo pipefail
cd "$(dirname "$0")"
code="${1:?usage: revoke-direct-code.sh <code>}"
target="${KV_TARGET:---remote}"
tmp="$(mktemp -d)"
trap 'command rm -rf "$tmp"' EXIT

npx wrangler kv key get --binding DIRECT_CODES registry:v1 "$target" >"$tmp/reg.json" 2>/dev/null || true
node --experimental-strip-types --input-type=module -e '
  import { readFileSync, writeFileSync } from "node:fs";
  import { revokeCode } from "./src/direct.ts";
  const [src, dst, code] = process.argv.slice(1);
  let reg = { accounts: {} };
  try { reg = JSON.parse(readFileSync(src, "utf8")); } catch { /* no registry yet */ }
  const out = revokeCode({ accounts: reg.accounts ?? {} }, code);
  writeFileSync(dst, JSON.stringify(out.reg));
  console.log(`accounts removed: ${out.removed}`);
' "$tmp/reg.json" "$tmp/out.json" "$code"
npx wrangler kv key put --binding DIRECT_CODES registry:v1 --path "$tmp/out.json" "$target" >/dev/null
npx wrangler kv key delete --binding DIRECT_CODES "$code" "$target" >/dev/null
echo "revoked $code: its devices lose Direct at the server's next sync (within ~10s)"
