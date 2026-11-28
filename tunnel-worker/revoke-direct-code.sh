#!/usr/bin/env bash
# revoke-direct-code.sh <code> — revoke a Direct code AND every device account it minted.
#
# Deleting the code alone used to be all revocation was, and it revoked nothing: every
# buyer held the one shared chisel secret. Accounts are per device now, and they live in
# DirectRegistry (src/entry.ts + src/direct.ts), so revoking removes them atomically via
# POST /direct/revoke when an admin/sync bearer is configured, falling back to local KV
# manipulation for --local rehearsals or pre-v2 deployments.
#
# Order matters: the accounts go first, the code second. If the first step fails, the
# code is still there and the whole thing can simply be run again.
#
#   DIRECT_ADMIN_TOKEN=... ./revoke-direct-code.sh gtd-<24hex>
#   KV_TARGET=--local ./revoke-direct-code.sh <code>   # rehearse against wrangler's local KV
set -euo pipefail
cd "$(dirname "$0")"
code="${1:?usage: revoke-direct-code.sh <code>}"
target="${KV_TARGET:---remote}"
api="${GTMUX_TUNNEL_API:-https://api.gtmux.ccy.dev}"
token="${DIRECT_ADMIN_TOKEN:-${DIRECT_SYNC_TOKEN:-}}"
tmp="$(mktemp -d)"
trap 'command rm -rf "$tmp"' EXIT

if [ "$target" != "--local" ] && [ -n "$token" ]; then
  payload="$(node -e 'console.log(JSON.stringify({ code: process.argv[1] }))' "$code")"
  status="$(curl -sS -o "$tmp/resp.json" -w '%{http_code}' -X POST "${api%/}/direct/revoke" \
    -H "Authorization: Bearer $token" \
    -H "Content-Type: application/json" \
    --data "$payload")"
  if [ "$status" = "200" ]; then
    node -e '
      const fs = require("node:fs");
      const out = JSON.parse(fs.readFileSync(process.argv[1], "utf8"));
      console.log(`accounts removed: ${out.removed ?? 0}`);
    ' "$tmp/resp.json"
    echo "revoked $code: its devices are cut off once their server's sync applies this and restarts chisel (normally within ~10s; check that server's gtmux-authsync log, see deploy/self-tunnel/README.md)"
    exit 0
  fi
  if [ "$status" != "404" ]; then
    echo "revoke-direct-code.sh: ${api%/}/direct/revoke failed (HTTP $status): $(cat "$tmp/resp.json")" >&2
    exit 1
  fi
fi

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
# Revoked here; cut off there only once the server's gtmux-authsync has applied the new
# file AND restarted chisel. A server that loses its LAST account applies that only with
# a gtmux-authsync that reads the provisioner's complete-set header.
echo "revoked $code: its devices are cut off once their server's sync applies this and restarts chisel (normally within ~10s; check that server's gtmux-authsync log, see deploy/self-tunnel/README.md)"
