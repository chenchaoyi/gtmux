#!/usr/bin/env bash
# Download only an authenticated, version-pinned installer. No third-party proxy.
set -euo pipefail
command -v openssl >/dev/null 2>&1 || { echo 'gtmux: OpenSSL or LibreSSL is required' >&2; exit 1; }
TMP_DIR="$(mktemp -d -t gtmux-bootstrap.XXXXXX)"
trap 'command rm -rf "$TMP_DIR"' EXIT
# Shared by the generated bootstrap and install.sh. Bash 3.2 compatible.
_feed_origins='https://downloads.ccy.dev
https://downloads.ccy.pub'
_feed_fetch() {
  local version="${1#v}" path origin compact payload signature result
  FEED_PAYLOAD=""
  command -v openssl >/dev/null 2>&1 || return 1
  path=stable.json
  if [ -n "$version" ]; then
    printf '%s\n' "$version" | LC_ALL=C grep -Eq '^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$' || return 1
    path="v${version}/manifest.json"
  fi
  cat > "$TMP_DIR/release-public.pem" <<'PUBLIC_KEY'
-----BEGIN PUBLIC KEY-----
MIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEA0cpoBjr42cKxTZtJOnUO
kIxyDITclBdsN6fxu5Q2h0FcSYw/EQJrbHVnbwsxcP8mq8pLXNgJJmygq0JoHXr+
KDX9R+7AJ/0ZdArqrcZ/gAe0Hyhdk9d3AuWVFyeePXUUkzUP3DVJ3dNyg/Y5IGnT
C+oYR0AThX2N5ZcZONNS6KyUPMpMLfmdAW6+dcU8W4Iz7Hx6rtnqT/X0U5udil3/
xAWEmAKBT4Hnfi4ZvZLut1eLn2awzVPBHrMWHqtGHiAPdPEwy22KYpY5PORw8E8H
5traGGIeBpCJESX8S65dh58ikAzLMaaNOfD2dcszfkvDm5ZE2XsO+xqQauCKlbO7
7u251f32T6Yh1zNRh8WDKEJKq5rmy6Xf2cPNt2F2BvM3JupFmIiLnWdisrhaCOcX
aq/ehII3wXcw0kb8JZU0gzi4CL8uaroWK4p5qsVaojIBLBT06u43+2EsFGUv/4Zr
uc9kZPR78VpJVleFRyrDptD0Lr6AYz+dVXklIRvIFFfrAgMBAAE=
-----END PUBLIC KEY-----
PUBLIC_KEY
  while IFS= read -r origin; do
    if curl --proto '=https' --max-time 12 --max-filesize 65536 -fsS "$origin/gtmux/$path" -o "$TMP_DIR/feed.json" 2>/dev/null; then :; else
      result=$?
      [ "$result" != 63 ] || return 2
      continue
    fi
    [ "$(wc -c < "$TMP_DIR/feed.json" | tr -d ' ')" -le 65536 ] || return 2
    compact="$(tr -d '\r\n' < "$TMP_DIR/feed.json")"
    payload="$(printf '%s' "$compact" | sed -nE 's/^\{"payload":"([A-Za-z0-9+/]+={0,2})","signature":"[A-Za-z0-9+/]+={0,2}"\}$/\1/p')"
    signature="$(printf '%s' "$compact" | sed -nE 's/^\{"payload":"[A-Za-z0-9+/]+={0,2}","signature":"([A-Za-z0-9+/]+={0,2})"\}$/\1/p')"
    [ -n "$payload" ] && [ -n "$signature" ] || return 2
    printf '%s' "$payload" | openssl base64 -d -A > "$TMP_DIR/feed-payload.json" 2>/dev/null || return 2
    printf '%s' "$signature" | openssl base64 -d -A > "$TMP_DIR/feed-signature" 2>/dev/null || return 2
    openssl dgst -sha256 -verify "$TMP_DIR/release-public.pem" -signature "$TMP_DIR/feed-signature" "$TMP_DIR/feed-payload.json" >/dev/null 2>&1 || return 2
    FEED_VERSION="$(awk -F'"' '$2=="version" {print $4;exit}' "$TMP_DIR/feed-payload.json")"
    printf '%s\n' "$FEED_VERSION" | LC_ALL=C grep -Eq '^(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})$' || return 2
    [ -z "$version" ] || [ "$FEED_VERSION" = "$version" ] || return 2
    FEED_PAYLOAD="$TMP_DIR/feed-payload.json"
    return 0
  done <<ORIGINS
$_feed_origins
ORIGINS
  return 1
}
_feed_meta() {
  [ -n "${FEED_PAYLOAD:-}" ] || return 1
  awk -F'"' -v n="$1" '$2==n {found=1;next} found&&$2=="sha256" {sha=$4} found&&$2=="size" {gsub(/[^0-9]/,"",$3);print sha" "$3;exit}' "$FEED_PAYLOAD"
}
_feed_check_file() {
  local file="$1" name="$2" sha size actual
  read -r sha size <<META
$(_feed_meta "$name")
META
  printf '%s\n' "$sha" | LC_ALL=C grep -Eq '^[0-9a-f]{64}$' || return 1
  case "$size" in ''|*[!0-9]*) return 1 ;; esac
  [ "${#size}" -le 9 ] && [ "$size" -gt 0 ] && [ "$size" -le 268435456 ] || return 1
  [ "$(wc -c < "$file" | tr -d ' ')" = "$size" ] || return 1
  actual="$(openssl dgst -sha256 -r "$file" | awk '{print $1}')" || return 1
  [ "$actual" = "$sha" ]
}
_feed_download() {
  local name="$1" dest="$2" origin sha size
  read -r sha size <<META
$(_feed_meta "$name")
META
  case "$size" in ''|*[!0-9]*) return 1 ;; esac
  [ "${#size}" -le 9 ] && [ "$size" -gt 0 ] && [ "$size" -le 268435456 ] || return 1
  while IFS= read -r origin; do
    if curl --proto '=https' --max-time 30 --max-filesize "$size" -fsS "$origin/gtmux/v$FEED_VERSION/$name" -o "$dest" 2>/dev/null && _feed_check_file "$dest" "$name"; then return 0; fi
  done <<ORIGINS
$_feed_origins
ORIGINS
  return 1
}

requested_version="${GTMUX_VERSION:-}"
if [ -n "$requested_version" ]; then
  printf '%s\n' "$requested_version" | LC_ALL=C grep -Eq '^v?(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})\.(0|[1-9][0-9]{0,8})([-+][0-9A-Za-z.-]+)?$' || { echo 'gtmux: invalid version pin' >&2; exit 1; }
  requested_version="v${requested_version#v}"
fi
if _feed_fetch "$requested_version"; then :; else
  result=$?
  # Older/unmirrored pins use a verified current installer, preserving the
  # original GitHub artifact channel. Invalid signed metadata never falls back.
  if [ "$result" = 1 ] && [ -n "$requested_version" ]; then
    if _feed_fetch ""; then result=0; else result=$?; fi
  fi
  if [ "$result" != 0 ]; then
  if [ "$result" = 2 ]; then echo 'gtmux: invalid release signature or metadata; stopped' >&2; else echo 'gtmux: official downloads unavailable; retry later or use the documented GitHub installation' >&2; fi
  exit 1
  fi
fi
_feed_download install.sh "$TMP_DIR/install.sh" || { echo 'gtmux: verified installer download failed; stopped' >&2; exit 1; }
GTMUX_VERSION="${requested_version:-v$FEED_VERSION}" bash "$TMP_DIR/install.sh" "$@"
