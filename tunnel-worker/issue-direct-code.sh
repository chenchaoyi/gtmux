#!/usr/bin/env bash
# Mint a paid "Direct" access code and store it in the DIRECT_CODES KV, then print it
# to hand to the buyer. The gtmux app/CLI redeems it via POST /direct/redeem.
#   ./issue-direct-code.sh "alice (afdian order #123)"
# Revoke:  npx wrangler kv key delete --binding DIRECT_CODES <code> --remote
# List:    npx wrangler kv key list   --binding DIRECT_CODES --remote
set -euo pipefail
cd "$(dirname "$0")"
LABEL="${1:-}"
CODE="gtd-$(openssl rand -hex 12)"   # gtd-<24hex>; matches the worker's ^[A-Za-z0-9-]{8,64}$
npx wrangler kv key put --binding DIRECT_CODES "$CODE" \
  "{\"label\":$(printf '%s' "$LABEL" | python3 -c 'import json,sys;print(json.dumps(sys.stdin.read()))')}" --remote >/dev/null
echo "$CODE"
