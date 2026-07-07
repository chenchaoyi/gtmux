#!/usr/bin/env bash
# Bring an existing xray VLESS-REALITY proxy back up and BEHIND the SNI router:
#   1. fix the "failed to load GeoIP: private" error (refresh geoip.dat)
#   2. move its VLESS inbound from public :443 → 127.0.0.1:8443 (nginx routes to it)
# Backs up the config and rolls back if xray -test fails. Idempotent. See README.md.
set -euo pipefail
CFG=/usr/local/etc/xray/config.json
DAT_DIR=/usr/local/share/xray
say() { echo "[xray-integrate] $*"; }
[ "$(id -u)" = 0 ] || { echo "run as root"; exit 1; }
[ -f "$CFG" ] || { echo "no xray config at $CFG"; exit 1; }

TS=$(date +%s)
cp -a "$CFG" "$CFG.bak-$TS"
say "backed up config → $CFG.bak-$TS"

# 1. Refresh geoip.dat / geosite.dat (the packaged one lacks the 'private' code).
install -d "$DAT_DIR"
for f in geoip.dat geosite.dat; do
  curl -fsSL -o "$DAT_DIR/$f.new" "https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/$f" \
    && mv "$DAT_DIR/$f.new" "$DAT_DIR/$f" && say "refreshed $f" || say "WARN: could not refresh $f"
done

# 2. Move the VLESS inbound to loopback:8443 (leave everything else untouched).
python3 - "$CFG" <<'PY'
import json, sys
p = sys.argv[1]
c = json.load(open(p))
moved = False
for i in c.get("inbounds", []):
    if i.get("protocol") == "vless" and i.get("port") in (443, "443"):
        i["port"] = 8443
        i["listen"] = "127.0.0.1"
        moved = True
json.dump(c, open(p, "w"), indent=2)
print("moved VLESS inbound → 127.0.0.1:8443" if moved else "no public :443 VLESS inbound found (already moved?)")
PY

# 3. Validate; roll back on failure.
if xray -test -config "$CFG" >/tmp/xray-test.log 2>&1; then
  systemctl restart xray
  sleep 1
  if systemctl is-active --quiet xray; then
    say "xray active on 127.0.0.1:8443 (VLESS-REALITY restored behind the SNI router)"
  else
    say "xray restarted but not active — see: journalctl -u xray -n 30"
  fi
else
  cp -a "$CFG.bak-$TS" "$CFG"
  say "xray -test FAILED, config rolled back. Details:"; cat /tmp/xray-test.log
  exit 1
fi
