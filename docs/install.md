# Install

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

Installs the checksum-verified binary to `~/.local/bin/gtmux`, plus the menu-bar
app. Options:

- `GTMUX_NO_APP=1` — skip the menu-bar app (CLI only).
- `GTMUX_APP_LOGIN=1` — start the app at login.
- `GTMUX_VERSION=vX.Y.Z` — pin a version.

From source:

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

Update later with `gtmux update`; remove the app with `gtmux uninstall-app`.

## China / unstable GitHub — mirror fallback

If even fetching the script fails (`raw.githubusercontent.com` blocked),
bootstrap from a CDN mirror. The script then mirror-falls-back its own downloads
automatically:

```sh
curl -fsSL https://cdn.jsdelivr.net/gh/chenchaoyi/gtmux@main/install.sh | bash
```

Once gtmux is installed, `gtmux update` fetches the script via the same mirror
list (jsdelivr → gh-proxy → ghfast → ghproxy), so updates work on CN networks
without any of this.

For asset downloads the installer is GitHub-first and auto-falls back to a mirror
chain (`ghfast.top` → `gh-proxy.com` → `ghproxy.net`) when a GitHub download
stalls. `SHASUMS256.txt` is always fetched GitHub-direct first, so the checksum
stays anchored on GitHub even when the tarball came through a mirror. Override
with `GTMUX_INSTALL_MIRROR`:

```sh
GTMUX_INSTALL_MIRROR=ghproxy  curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash   # straight to the mirror chain
GTMUX_INSTALL_MIRROR=https://my.mirror/  curl -fsSL ... | bash   # custom <prefix><github-url> proxy
GTMUX_INSTALL_MIRROR=github   curl -fsSL ... | bash   # GitHub only, no mirrors
```

## Signing & permissions

macOS ties granted permissions to the app's code signature. A **Developer
ID-signed + notarized** build keeps your grants across updates; an **ad-hoc**
build (a local `make app`, or an unsigned release) changes identity every build,
so macOS forgets and re-prompts. Set `GTMUX_SIGN_ID` when building to sign with
your Developer ID (see `macapp/build.sh`).
