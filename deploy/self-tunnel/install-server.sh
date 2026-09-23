#!/usr/bin/env bash
# Idempotent installer for the gtmux self-hosted tunnel server (Debian 12), on a
# DEDICATED VPS where Caddy can own :443. Sets up Caddy (TLS on :443, ACME on :80)
# + chisel (reverse tunnel on loopback). Re-runnable. See README.md.
#
# Two modes:
#   personal (default)  one shared chisel secret, for a server with a single tenant
#   Direct              DIRECT_SYNC_TOKEN=<token> bash install-server.sh
#                       the operated, multi-tenant server: one account per device, each
#                       allowed only its own port, synced from the provisioner Worker.
#                       Once a server is in Direct mode it stays there on re-runs.
#
#   CADDY=skip          leave Caddy exactly as it is. For a box whose :443 is fronted by
#                       something else (an SNI router) with its own host-specific Caddyfile:
#                       installing this directory's Caddyfile there would take :443 from
#                       that router and restart Caddy onto a port it cannot bind.
#
# Front ends (who terminates TLS on :443):
#   FRONT=caddy         the default: Caddy owns the site (with CADDY=skip when something
#                       else fronts it).
#   FRONT=nginx         a box whose :443 already belongs to an existing nginx serving other
#                       sites. Caddy is not installed at all; one extra nginx site is added
#                       for DOMAIN and every other site is left alone. Needs DOMAIN=<host>.
#                       nginx sees the visitor directly, so the device roster shows real
#                       addresses here.
#
# Getting chisel onto a box that cannot reach GitHub (measured on a mainland host: the
# release download truncates or times out):
#   CHISEL_MIRROR=https://gh-proxy.com/   a prefix put in front of the GitHub URL
#   CHISEL_BIN=/path/to/chisel            a binary you carried over yourself
# Either way the pinned SHA-256 is checked before anything is installed: the trust is in
# the checksum, never in whoever served the bytes.
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
say() { echo "[gtmux-self-tunnel] $*"; }

[ "$(id -u)" = 0 ] || { echo "run as root"; exit 1; }

# --- packages -----------------------------------------------------------------
export DEBIAN_FRONTEND=noninteractive
say "installing chisel + caddy…"
apt-get -qq update >/dev/null
# Only what this run actually needs. The keyrings and apt-transport-https exist for Caddy's
# apt repository, so a box that will never install Caddy does not get them.
apt-get -y -qq install curl >/dev/null

# Pinned, and UPGRADED when it differs: this used to install only when chisel was absent,
# so a re-run never took a fix. At least 1.11.5 is required in Direct mode, where accounts
# are restricted: GO-2026-5054 (fixed in 1.11.5) is an ACL bypass.
#
# 1.12.0, not the 1.12.1 the CLI links: 1.12.1 is a module tag with no GitHub release, so
# there is no binary to download (the first cutover attempt stopped on a 404 here, before
# changing anything). The two differ only in client-side UDP forwarding; every server file
# that authenticates, applies the authfile or checks a channel is identical.
V=1.12.0
CHISEL_SHA256=f3f180f1d93aa72cce4e6386f98cc06569a0146fbd65eb4423cf83e6434bcfe6 # chisel_1.12.0_linux_amd64.gz
FRONT="${FRONT:-caddy}"

# fetch_chisel puts the pinned, VERIFIED binary in place, from whichever source can reach
# it. A mirror is fine because the checksum is the thing being trusted; a file that does
# not match it is never installed, whatever served it.
fetch_chisel() {
  local url="https://github.com/jpillora/chisel/releases/download/v${V}/chisel_${V}_linux_amd64.gz"
  local tmp=/tmp/gtmux-chisel.$$.gz got
  if [ -n "${CHISEL_BIN:-}" ]; then
    [ -x "$CHISEL_BIN" ] || { echo "CHISEL_BIN=$CHISEL_BIN is not an executable file"; exit 1; }
    install -m 755 "$CHISEL_BIN" /usr/local/bin/chisel
    say "chisel: installed from $CHISEL_BIN"
    return
  fi
  for src in ${CHISEL_MIRROR:+"${CHISEL_MIRROR%/}/$url"} "$url"; do
    command rm -f "$tmp"
    curl -fsSL --max-time 120 -o "$tmp" "$src" || { say "chisel: no luck from ${src%%/jpillora*}…"; continue; }
    if ! "$HERE/verify-download.sh" "$tmp" "$CHISEL_SHA256" >/dev/null; then
      say "chisel: WRONG CHECKSUM from ${src%%/jpillora*}…; ignoring what it served"
      continue
    fi
    gunzip -f "$tmp" && chmod +x "${tmp%.gz}" && mv "${tmp%.gz}" /usr/local/bin/chisel
    say "chisel: downloaded and checksum verified"
    return
  done
  echo "could not get chisel ${V} with a matching checksum. Pass CHISEL_MIRROR=<prefix> or CHISEL_BIN=<path>."
  exit 1
}

if ! command -v chisel >/dev/null || [ "$(chisel --version 2>/dev/null)" != "$V" ]; then
  fetch_chisel
fi
say "chisel: $(chisel --version)"

if [ "$FRONT" = caddy ]; then
  if ! command -v caddy >/dev/null; then
    apt-get -y -qq install gnupg debian-keyring debian-archive-keyring apt-transport-https >/dev/null
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' > /etc/apt/sources.list.d/caddy-stable.list
    apt-get -qq update >/dev/null && apt-get -y -qq install caddy >/dev/null
  fi
  say "caddy: $(caddy version)"
else
  command -v nginx >/dev/null || { echo "FRONT=nginx but nginx is not installed here"; exit 1; }
  [ -n "${DOMAIN:-}" ] || { echo "FRONT=nginx needs DOMAIN=<the host name this server answers on>"; exit 1; }
  say "front: the nginx already on this box (${DOMAIN})"
fi

MODE=personal
if [ -n "${DIRECT_SYNC_TOKEN:-}" ] || [ -s /etc/gtmux-tunnel/sync.env ]; then
  MODE=direct
fi
say "mode: $MODE"

# --- shared secret (personal mode only; generate once) -------------------------
install -d -m 700 /etc/gtmux-tunnel
if [ "$MODE" = direct ]; then
  : # no shared secret: see the Direct section below
elif [ ! -s /etc/gtmux-tunnel/chisel.env ]; then
  SECRET="gtmux:$(head -c 24 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)"
  printf 'AUTH=%s\n' "$SECRET" > /etc/gtmux-tunnel/chisel.env
  chmod 600 /etc/gtmux-tunnel/chisel.env
  say "generated chisel secret → /etc/gtmux-tunnel/chisel.env (copy AUTH to the Mac)"
else
  say "chisel secret already present (kept)"
fi

# --- chisel server service ----------------------------------------------------
install -m 644 "$HERE/chisel-server.service" /etc/systemd/system/chisel-server.service

# --- Direct mode: one account per device, synced from the provisioner ------------
if [ "$MODE" = direct ]; then
  apt-get -y -qq install jq >/dev/null
  id gtmux-tunnel >/dev/null 2>&1 || useradd --system --no-create-home --shell /usr/sbin/nologin gtmux-tunnel
  # chisel runs as gtmux-tunnel and must read (and watch) this directory; sync.env and any
  # retired secret inside stay root-only.
  chown root:gtmux-tunnel /etc/gtmux-tunnel
  chmod 750 /etc/gtmux-tunnel
  if [ -n "${DIRECT_SYNC_TOKEN:-}" ]; then
    umask 077
    printf 'SYNC_URL=%s\nSYNC_TOKEN=%s\n' \
      "${DIRECT_SYNC_URL:-https://api.gtmux.ccy.dev/direct/authfile}" "$DIRECT_SYNC_TOKEN" \
      >/etc/gtmux-tunnel/sync.env
    umask 022
  fi
  # The sentinel keeps the authfile from ever being empty, which chisel would read as
  # "no authentication" (see gtmux-authsync). Its password exists only here.
  if [ ! -s /etc/gtmux-tunnel/sentinel ]; then
    umask 077
    printf 'gtmux-sentinel:%s\n' "$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')" >/etc/gtmux-tunnel/sentinel
    umask 022
  fi
  # Whatever users.json holds now, it holds the sentinel before chisel reads it: an older
  # file without one, with the first sync below failing, would otherwise start chisel open.
  base='{}'
  [ -s /etc/gtmux-tunnel/users.json ] && base="$(cat /etc/gtmux-tunnel/users.json)"
  printf '%s' "$base" | jq --arg s "$(cat /etc/gtmux-tunnel/sentinel)" '. + {($s): ["^$"]}' \
    >/etc/gtmux-tunnel/users.json.new
  chown gtmux-tunnel:gtmux-tunnel /etc/gtmux-tunnel/users.json.new
  chmod 600 /etc/gtmux-tunnel/users.json.new
  mv -f /etc/gtmux-tunnel/users.json.new /etc/gtmux-tunnel/users.json
  install -m 755 "$HERE/gtmux-authsync" /usr/local/bin/gtmux-authsync
  install -m 644 "$HERE/gtmux-authsync.service" /etc/systemd/system/gtmux-authsync.service
  install -m 644 "$HERE/gtmux-authsync.timer" /etc/systemd/system/gtmux-authsync.timer
  install -d -m 755 /etc/systemd/system/chisel-server.service.d
  install -m 644 "$HERE/chisel-server.direct.conf" /etc/systemd/system/chisel-server.service.d/direct.conf
  systemctl daemon-reload
  # Fill the authfile BEFORE chisel restarts on it, or every device waits for the timer.
  if /usr/local/bin/gtmux-authsync; then
    say "authfile: $(( $(jq length /etc/gtmux-tunnel/users.json) - 1 )) device account(s), plus the sentinel"
  else
    say "authfile: first sync FAILED (check SYNC_URL/SYNC_TOKEN and the Worker's DIRECT_SYNC_TOKEN)"
  fi
  systemctl enable --now gtmux-authsync.timer >/dev/null
  # The shared secret is what this mode exists to end. Nothing reads it any more; the
  # rename keeps it for the record without leaving a working credential lying around.
  if [ -f /etc/gtmux-tunnel/chisel.env ]; then
    mv -f /etc/gtmux-tunnel/chisel.env /etc/gtmux-tunnel/chisel.env.retired
  fi
fi

systemctl daemon-reload
systemctl enable chisel-server >/dev/null
systemctl restart chisel-server
say "chisel-server: $(systemctl is-active chisel-server) on 127.0.0.1:8080 ($MODE mode)"

# --- the front end ------------------------------------------------------------
if [ "$FRONT" = nginx ]; then
  # One extra site, for DOMAIN only. Every other site on this box is left untouched, and
  # nginx is reloaded, never restarted, so the sites already being served keep serving.
  site=/etc/nginx/sites-available/gtmux-direct.conf
  cert=/etc/letsencrypt/live/${DOMAIN}/fullchain.pem
  if [ -f "$cert" ]; then
    src="$HERE/nginx-site.conf"; phase="with TLS"
  else
    src="$HERE/nginx-site-acme.conf"; phase="HTTP only, so certbot has somewhere to attach"
  fi
  sed "s/__DOMAIN__/${DOMAIN}/g" "$src" >"$site"
  ln -sf "$site" /etc/nginx/sites-enabled/gtmux-direct.conf
  if ! nginx -t 2>/tmp/gtmux-nginx-t.log; then
    cat /tmp/gtmux-nginx-t.log
    command rm -f /etc/nginx/sites-enabled/gtmux-direct.conf
    nginx -t >/dev/null 2>&1 && systemctl reload nginx
    echo "the gtmux site was REMOVED again and nginx left as it was"
    exit 1
  fi
  systemctl reload nginx
  say "nginx: gtmux-direct.conf installed for ${DOMAIN} (${phase})"
  if [ ! -f "$cert" ]; then
    say "NEXT, yours to run: certbot --nginx -d ${DOMAIN}   (then re-run this script to serve it)"
  fi
elif [ "${CADDY:-}" = skip ]; then
  say "caddy left as it is (CADDY=skip): $(systemctl is-active caddy)"
else
  install -m 644 "$HERE/Caddyfile" /etc/caddy/Caddyfile
  systemctl enable caddy >/dev/null
  systemctl restart caddy
  say "caddy restarted, owns :443 (ACME pends until the host name resolves to this box)"
fi

say "DONE. The host name must resolve to this box before a certificate can be issued."
