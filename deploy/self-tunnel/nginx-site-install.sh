#!/usr/bin/env bash
# nginx-site-install.sh SRC DOMAIN — install-server.sh's FRONT=nginx step: put the gtmux
# site for DOMAIN in place, check the whole nginx config, and reload nginx. When the check
# fails, it puts back exactly what was there before (the site file and its enabled link,
# or neither) and says what state nginx is in.
#
# A re-run used to overwrite the existing site, often the one certbot had edited, and on
# a failed check remove the enabled link and reload: the gtmux site that WAS serving went
# down, while the message said "nginx left as it was" (%12, 2026-10-06).
#
# The overrides exist so this can be exercised off a server; production sets none.
set -euo pipefail
src="$1"
domain="$2"
etc="${NGINX_ETC:-/etc/nginx}"
nginx_bin="${NGINX_BIN:-nginx}"
reload="${NGINX_RELOAD:-systemctl reload nginx}"
site="$etc/sites-available/gtmux-direct.conf"
link="$etc/sites-enabled/gtmux-direct.conf"

# What is there now, kept outside every directory nginx includes.
bak="$(mktemp -d)"
trap 'command rm -rf "$bak"' EXIT
if [ -e "$site" ]; then command cp -p "$site" "$bak/site"; fi
if [ -L "$link" ]; then
  readlink "$link" >"$bak/link-target"
elif [ -e "$link" ]; then
  command cp -p "$link" "$bak/link-file"
fi

sed "s/__DOMAIN__/${domain}/g" "$src" >"$site"
ln -sf "$site" "$link"
if ! "$nginx_bin" -t 2>"$bak/nginx-t.log"; then
  cat "$bak/nginx-t.log"
  if [ -e "$bak/site" ]; then command cp -p "$bak/site" "$site"; else command rm -f "$site"; fi
  command rm -f "$link"
  if [ -e "$bak/link-target" ]; then
    ln -s "$(cat "$bak/link-target")" "$link"
  elif [ -e "$bak/link-file" ]; then
    command cp -p "$bak/link-file" "$link"
  fi
  if "$nginx_bin" -t >/dev/null 2>&1; then
    $reload
    echo "the new gtmux site failed nginx -t; the previous gtmux site (or none) is back in place and nginx reloaded on it"
  else
    echo "the new gtmux site failed nginx -t; the previous gtmux site (or none) is back in place, but nginx -t still fails without it, so nginx was NOT reloaded"
  fi
  exit 1
fi
$reload
