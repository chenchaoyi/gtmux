#!/usr/bin/env bash
# gtmux one-line installer.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
#
# Pin a version:
#   GTMUX_VERSION=v0.1.0 curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
#
# CN networks (GitHub asset downloads often stall on objects.githubusercontent.com):
#   the installer is GitHub-first and AUTO-falls back to a mirror chain. Force a mode:
#   GTMUX_INSTALL_MIRROR=ghproxy            # skip the GitHub-first attempt, go to mirrors
#   GTMUX_INSTALL_MIRROR=https://my.proxy/  # a custom <prefix><github-url> proxy
#   GTMUX_INSTALL_MIRROR=github             # GitHub only, no mirrors
#
# What it does:
#   1. Confirms host is macOS arm64 / x86_64
#   2. Resolves the release version (default: latest tag)
#   3. Downloads the matching tarball + SHASUMS256.txt, verifies SHA256
#   4. Installs the CLI binary to ~/.local/bin/gtmux (atomic swap)
#   5. Installs/updates the menu-bar app (~/Applications/Gtmux.app) and launches
#      it — skip with GTMUX_NO_APP=1
#   6. Prints a PATH hint if ~/.local/bin isn't on PATH
#
# Trust: SHASUMS256.txt is always tried GitHub-direct first, so even when the
# tarball comes from a mirror the checksum it's verified against is anchored on
# GitHub. macOS ships bash 3.2 — this script stays 3.2-compatible.

set -euo pipefail

REPO_SLUG="${GTMUX_REPO:-chenchaoyi/gtmux}"
BIN_DIR="${GTMUX_BIN_DIR:-${HOME}/.local/bin}"
TMP_DIR=""

# ---- color (TTY only) ----
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-}" != "dumb" ]; then
  GREEN="$(printf '\033[32m')"; RED="$(printf '\033[31m')"
  DIM="$(printf '\033[2m')"; BOLD="$(printf '\033[1m')"; RESET="$(printf '\033[0m')"
else
  GREEN="" RED="" DIM="" BOLD="" RESET=""
fi

die()  { echo "${RED}gtmux install: error:${RESET} $*" >&2; exit 1; }
note() { echo "gtmux install: $*"; }
step() { printf '%s[%s/%s]%s %-9s %s %s✓%s\n' "$DIM" "$1" "$STEPS" "$RESET" "$2" "$3" "$GREEN" "$RESET"; }
# Numbered steps: 5 with the menu-bar app, 4 when GTMUX_NO_APP skips it.
STEPS=5; [ -n "${GTMUX_NO_APP:-}" ] && STEPS=4

cleanup() { [ -n "$TMP_DIR" ] && [ -d "$TMP_DIR" ] && rm -rf "$TMP_DIR"; }
trap cleanup EXIT

# detect_locale — `zh` if macOS-preferred language starts with zh, else `en`.
detect_locale() {
  local pref
  pref="$(defaults read -g AppleLanguages 2>/dev/null | tr -d '(),"' | awk 'NF{print $1; exit}')"
  case "$pref" in zh*) echo zh ;; *) echo en ;; esac
}
LOCALE="$(detect_locale)"

# ---- mirror resolution ----
# auto    — GitHub first, then walk the proxy chain on failure (default)
# github  — GitHub only
# ghproxy — skip GitHub-first for the tarball, go straight to the proxy chain
# <url>   — a custom http(s):// proxy prefix used as <prefix><github-url>
# SHASUMS is ALWAYS tried GitHub-direct first regardless of mode (trust anchor).
_MIRROR_PROXY_CHAIN='https://ghfast.top/
https://gh-proxy.com/
https://ghproxy.net/'

case "${GTMUX_INSTALL_MIRROR:-auto}" in
  auto)    MIRROR=auto;   MIRROR_PROXIES="$_MIRROR_PROXY_CHAIN" ;;
  github)  MIRROR=github; MIRROR_PROXIES="" ;;
  ghproxy) MIRROR=ghproxy; MIRROR_PROXIES="$_MIRROR_PROXY_CHAIN" ;;
  http://*|https://*)
    MIRROR=custom; MIRROR_PROXIES="${GTMUX_INSTALL_MIRROR%/}/" ;;
  *)
    die "unknown GTMUX_INSTALL_MIRROR: ${GTMUX_INSTALL_MIRROR} (expected auto|github|ghproxy|<http(s)://proxy/>)" ;;
esac

_mirror_host() { local h="${1#http://}"; h="${h#https://}"; echo "${h%%/*}"; }

mirror_fallback_notice() {
  local host; host="$(_mirror_host "$1")"
  case "$LOCALE" in
    zh) echo "GitHub 下载失败(网络可能受限);切换到 ${host} 镜像重试..." ;;
    *)  echo "GitHub download failed (likely CN network); retrying via ${host} mirror..." ;;
  esac
}
mirror_completion_notice() {
  local tsrc="$1" ssrc="$2" thost shost
  thost="$(_mirror_host "$tsrc")"; shost="$(_mirror_host "$ssrc")"
  if [ "$ssrc" = github ]; then
    case "$LOCALE" in
      zh) echo "tarball 经 ${thost} 镜像;SHASUMS 从 GitHub 直连,信任锚定 GitHub" ;;
      *)  echo "tarball via ${thost} mirror; SHASUMS direct from GitHub — trust anchored on GitHub" ;;
    esac
  else
    case "$LOCALE" in
      zh) echo "SHASUMS 经 ${shost} 镜像;信任锚定该镜像(SHA256 仍校验)" ;;
      *)  echo "SHASUMS via ${shost} mirror; trust anchored on that mirror (SHA256 still verified)" ;;
    esac
  fi
}

# validate_download — guard against a proxy that 200s an HTML landing page.
validate_download() {
  local file="$1" kind="$2"
  [ -s "$file" ] || return 1
  case "$kind" in
    shasums) awk -v n="$TARBALL_NAME" '$2 ~ n && $1 ~ /^[0-9a-fA-F]{64}$/ {ok=1} END{exit ok?0:1}' "$file" ;;
    tarball) gzip -t "$file" 2>/dev/null ;;
    zip)     unzip -tqq "$file" >/dev/null 2>&1 ;; # rejects an HTML landing page
    *) return 0 ;;
  esac
}

# build_candidates — emit "src url" lines in priority order for the mode.
build_candidates() {
  local url="$1" force_gh="$2" p
  if [ "$MIRROR" = github ] || [ "$MIRROR" = auto ] || [ "$MIRROR" = custom ] || [ "$force_gh" = 1 ]; then
    printf 'github %s\n' "$url"
  fi
  if [ "$MIRROR" != github ]; then
    printf '%s\n' "$MIRROR_PROXIES" | while IFS= read -r p; do
      [ -z "$p" ] && continue
      printf '%s %s%s\n' "${p%/}" "$p" "$url"
    done
  fi
}

# download_with_fallback <url> <dest> <used_var> [kind] [force_gh]
# Walks the candidate chain; accepts the first attempt that transfers AND passes
# the content check. Writes the winning src into the named global (bash 3.2 eval).
download_with_fallback() {
  local url="$1" dest="$2" used_var="$3" kind="${4:-plain}" force_gh="${5:-0}"
  local src curl_url
  while IFS=' ' read -r src curl_url; do
    [ -z "$curl_url" ] && continue
    [ "$src" != github ] && note "$(mirror_fallback_notice "$src")"
    if curl --max-time 20 -fsSL --output "$dest" "$curl_url" 2>/dev/null && validate_download "$dest" "$kind"; then
      eval "${used_var}=\"\$src\""
      return 0
    fi
    rm -f "$dest"
  done <<EOF
$(build_candidates "$url" "$force_gh")
EOF
  return 1
}

# resolve_latest_version — GitHub API fast path, else follow the
# releases/latest redirect through the candidate chain (proxies relay it).
resolve_latest_version() {
  local v src curl_url final
  v="$(curl --max-time 10 -fsSL "https://api.github.com/repos/${REPO_SLUG}/releases/latest" 2>/dev/null \
        | awk -F'"' '/"tag_name"/ {print $4; exit}')" || true
  if [ -n "$v" ]; then printf '%s\n' "$v"; return 0; fi
  while IFS=' ' read -r src curl_url; do
    [ -z "$curl_url" ] && continue
    [ "$src" != github ] && note "$(mirror_fallback_notice "$src")" >&2
    final="$(curl --max-time 20 -fsSL -o /dev/null -w '%{url_effective}' "$curl_url" 2>/dev/null)" || continue
    case "$final" in
      */tag/*)
        v="${final##*/tag/}"
        case "$v" in v[0-9]*|[0-9]*) printf '%s\n' "$v"; return 0 ;; esac ;;
    esac
  done <<EOF
$(build_candidates "https://github.com/${REPO_SLUG}/releases/latest" 0)
EOF
  return 1
}

# ---- platform check ----
OS="$(uname -s)"; ARCH_RAW="$(uname -m)"
[ "$OS" = "Darwin" ] || die "gtmux is macOS-only (uname -s = $OS)"
case "$ARCH_RAW" in
  arm64|aarch64) ARCH="arm64" ;;
  x86_64|amd64)  ARCH="amd64" ;;
  *) die "unsupported arch: $ARCH_RAW (gtmux ships arm64 + amd64 builds)" ;;
esac

printf '\n  %sgtmux%s installer\n\n' "$BOLD" "$RESET"
step 1 "Host" "darwin-${ARCH}"

# ---- resolve version ----
if [ -n "${GTMUX_VERSION:-}" ]; then
  VERSION="$GTMUX_VERSION"
else
  VERSION="$(resolve_latest_version)" || true
  [ -n "$VERSION" ] || die "could not resolve latest release (GitHub API + mirror chain all failed); pin one with GTMUX_VERSION=vX.Y.Z, or see GTMUX_INSTALL_MIRROR options"
fi
step 2 "Release" "$VERSION"

NUM_VERSION="${VERSION#v}"
TARBALL_NAME="gtmux-${NUM_VERSION}-darwin-${ARCH}.tar.gz"
TARBALL_URL="https://github.com/${REPO_SLUG}/releases/download/${VERSION}/${TARBALL_NAME}"
SHASUMS_URL="https://github.com/${REPO_SLUG}/releases/download/${VERSION}/SHASUMS256.txt"

# ---- download + verify ----
TMP_DIR="$(mktemp -d -t gtmux-install.XXXXXX)"
TARBALL_MIRROR=""; SHASUMS_MIRROR=""
download_with_fallback "$TARBALL_URL" "${TMP_DIR}/${TARBALL_NAME}" TARBALL_MIRROR tarball 0 \
  || die "tarball download failed — GitHub and every mirror exhausted: $TARBALL_URL"
download_with_fallback "$SHASUMS_URL" "${TMP_DIR}/SHASUMS256.txt" SHASUMS_MIRROR shasums 1 \
  || die "SHASUMS256.txt download failed — GitHub and every mirror exhausted: $SHASUMS_URL"

EXPECTED_SHA="$(awk -v n="$TARBALL_NAME" '$2 ~ n {print $1}' "${TMP_DIR}/SHASUMS256.txt" | head -1)"
ACTUAL_SHA="$(shasum -a 256 "${TMP_DIR}/${TARBALL_NAME}" | awk '{print $1}')"
[ "$ACTUAL_SHA" = "$EXPECTED_SHA" ] || die "sha256 mismatch on $TARBALL_NAME (expected $EXPECTED_SHA, got $ACTUAL_SHA)"
step 3 "Verify" "sha256 ${ACTUAL_SHA:0:8}…"
if [ "$TARBALL_MIRROR" != github ] || [ "$SHASUMS_MIRROR" != github ]; then
  note "$(mirror_completion_notice "$TARBALL_MIRROR" "$SHASUMS_MIRROR")"
fi

# ---- install (atomic swap) ----
mkdir -p "$BIN_DIR"
tar -xzf "${TMP_DIR}/${TARBALL_NAME}" -C "$TMP_DIR"
[ -x "${TMP_DIR}/gtmux" ] || die "extracted tarball missing the gtmux binary"
mv "${TMP_DIR}/gtmux" "${BIN_DIR}/gtmux.new"
chmod +x "${BIN_DIR}/gtmux.new"
mv -f "${BIN_DIR}/gtmux.new" "${BIN_DIR}/gtmux"
# curl-delivered files get a quarantine xattr; strip it so first run isn't blocked.
xattr -d com.apple.quarantine "${BIN_DIR}/gtmux" 2>/dev/null || true
step 4 "Install" "${BIN_DIR}/gtmux"

# ---- menu-bar app (~/Applications/Gtmux.app) ----
# Shipped as a separate universal, ad-hoc-signed bundle (cgo; the CLI is cgo-free).
# It's NOT in SHASUMS256.txt (a separate CI job uploads it), so we validate the
# zip structurally rather than by checksum. Opt out with GTMUX_NO_APP=1.
APP_DIR="${HOME}/Applications"
LSREGISTER="/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister"
if [ -z "${GTMUX_NO_APP:-}" ]; then
  APP_ZIP="Gtmux-${NUM_VERSION}-macos.zip"
  APP_URL="https://github.com/${REPO_SLUG}/releases/download/${VERSION}/${APP_ZIP}"
  APP_MIRROR=""
  if download_with_fallback "$APP_URL" "${TMP_DIR}/${APP_ZIP}" APP_MIRROR zip 0 \
     && rm -rf "${TMP_DIR}/app" && mkdir -p "${TMP_DIR}/app" \
     && ditto -x -k "${TMP_DIR}/${APP_ZIP}" "${TMP_DIR}/app" 2>/dev/null \
     && [ -d "${TMP_DIR}/app/Gtmux.app" ]; then
    mkdir -p "$APP_DIR"
    rm -rf "${APP_DIR}/Gtmux.app.new"
    mv "${TMP_DIR}/app/Gtmux.app" "${APP_DIR}/Gtmux.app.new"
    # Stop a running (old) instance so the swap + relaunch picks up the new build.
    pkill -f 'Gtmux.app/Contents/MacOS/gtmux-menubar' 2>/dev/null || true
    rm -rf "${APP_DIR}/Gtmux.app"
    mv "${APP_DIR}/Gtmux.app.new" "${APP_DIR}/Gtmux.app"   # same-fs rename (atomic)
    xattr -dr com.apple.quarantine "${APP_DIR}/Gtmux.app" 2>/dev/null || true
    "$LSREGISTER" -f "${APP_DIR}/Gtmux.app" 2>/dev/null || true
    open "${APP_DIR}/Gtmux.app" 2>/dev/null || true
    step 5 "Menu bar" "${APP_DIR}/Gtmux.app"
  else
    note "menu-bar app: download/unzip failed — CLI is installed; retry or skip with GTMUX_NO_APP=1"
  fi
fi

# ---- PATH hint ----
printf '\n  %sInstalled.%s  %sgtmux %s%s\n' "$BOLD" "$RESET" "$DIM" "$VERSION" "$RESET"
if [ -z "${GTMUX_NO_APP:-}" ] && [ -d "${APP_DIR}/Gtmux.app" ]; then
  case "$LOCALE" in
    zh) printf '  菜单栏 app 已启动 — 看右上角的彩色圆点(左键点开)\n' ;;
    *)  printf '  menu-bar app launched — look for the colored dot up top (left-click it)\n' ;;
  esac
fi
case ":$PATH:" in
  *":${BIN_DIR}:"*)
    case "$LOCALE" in
      zh) printf '  下一步: 运行 %sgtmux%s 看用法\n\n' "$BOLD" "$RESET" ;;
      *)  printf '  next: run %sgtmux%s for usage\n\n' "$BOLD" "$RESET" ;;
    esac ;;
  *)
    SHELL_NAME="$(basename "${SHELL:-/bin/zsh}")"
    case "$SHELL_NAME" in
      zsh)  RC="~/.zshrc";  LINE="export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
      bash) RC="~/.bashrc"; LINE="export PATH=\"\$HOME/.local/bin:\$PATH\"" ;;
      fish) RC="~/.config/fish/config.fish"; LINE="fish_add_path \$HOME/.local/bin" ;;
      *)    RC=""; LINE="" ;;
    esac
    if [ -n "$RC" ]; then
      case "$LOCALE" in
        zh) printf '  下一步: 把这行加到 %s:\n    %s\n\n' "$RC" "$LINE" ;;
        *)  printf '  next: add to %s:\n    %s\n\n' "$RC" "$LINE" ;;
      esac
    else
      printf '  next: add %s to your PATH (shell: %s)\n\n' "$BIN_DIR" "$SHELL_NAME"
    fi ;;
esac
