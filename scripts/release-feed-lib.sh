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
@PUBLIC_KEY@
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
