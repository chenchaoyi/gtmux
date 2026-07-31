#!/usr/bin/env bash
# Idempotent installer for the gtmux self-hosted tunnel server (Debian 12), on a
# DEDICATED VPS where Caddy can own :443. Sets up Caddy (TLS on :443, ACME on :80)
# + chisel (reverse tunnel on loopback). Re-runnable. See README.md.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
say() { echo "[gtmux-self-tunnel] $*"; }

[ "$(id -u)" = 0 ] || { echo "run as root"; exit 1; }

# --- packages -----------------------------------------------------------------
export DEBIAN_FRONTEND=noninteractive
say "installing chisel + caddy…"
apt-get -qq update >/dev/null
apt-get -y -qq install curl gnupg debian-keyring debian-archive-keyring apt-transport-https >/dev/null

if ! command -v chisel >/dev/null; then
  V=1.10.1
  curl -fsSL -o /tmp/chisel.gz "https://github.com/jpillora/chisel/releases/download/v${V}/chisel_${V}_linux_amd64.gz"
  gunzip -f /tmp/chisel.gz && chmod +x /tmp/chisel && mv /tmp/chisel /usr/local/bin/chisel
fi
say "chisel: $(chisel --version)"

if ! command -v caddy >/dev/null; then
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
  curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' > /etc/apt/sources.list.d/caddy-stable.list
  apt-get -qq update >/dev/null && apt-get -y -qq install caddy >/dev/null
fi
say "caddy: $(caddy version)"

# --- shared secret (generate once) --------------------------------------------
install -d -m 700 /etc/gtmux-tunnel
if [ ! -s /etc/gtmux-tunnel/chisel.env ]; then
  SECRET="gtmux:$(head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)"
  printf 'AUTH=%s\n' "$SECRET" > /etc/gtmux-tunnel/chisel.env
  chmod 600 /etc/gtmux-tunnel/chisel.env
  say "generated chisel secret → /etc/gtmux-tunnel/chisel.env (copy AUTH to the Mac)"
else
  say "chisel secret already present (kept)"
fi

# --- chisel server service ----------------------------------------------------
install -m 644 "$HERE/chisel-server.service" /etc/systemd/system/chisel-server.service
systemctl daemon-reload
systemctl enable --now chisel-server >/dev/null
say "chisel-server: $(systemctl is-active chisel-server) on 127.0.0.1:8080"

# --- Caddy (owns :443 + :80 directly) -----------------------------------------
install -m 644 "$HERE/Caddyfile" /etc/caddy/Caddyfile
systemctl enable caddy >/dev/null
systemctl restart caddy
say "caddy restarted, owns :443 (ACME pends until tunnel.ccy.dev resolves to this host)"

say "DONE. Next: add DNS tunnel.ccy.dev → this IP (DNS-only) so Caddy can issue the cert."
