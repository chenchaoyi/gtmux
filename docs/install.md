# Install

**English** · [中文](install.zh.md)

## Homebrew (macOS)

```sh
brew install chenchaoyi/tap/gtmux             # the CLI
brew install --cask chenchaoyi/tap/gtmux-app  # the menu-bar app (optional)
```

Or tap once, then install by name:

```sh
brew tap chenchaoyi/tap
brew install gtmux
brew install --cask gtmux-app
```

Upgrade with `brew upgrade gtmux` (and `brew upgrade --cask gtmux-app`). The CLI
installs as a Homebrew cask. Without Homebrew, use the install script below.

## Install script

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
```

Installs the checksum-verified binary to `~/.local/bin/gtmux`, plus the menu-bar
app. Options:

- `GTMUX_NO_APP=1`: skip the menu-bar app (CLI only).
- `GTMUX_APP_LOGIN=1`: start the app at login.
- `GTMUX_VERSION=vX.Y.Z`: pin a version.

From source:

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

Update later with `gtmux update`. To remove: `gtmux uninstall app` takes the
menu-bar app off, `gtmux uninstall hooks` unregisters the agent hooks, and
`gtmux uninstall all` does both.

## China / unstable GitHub: mirror fallback

If even fetching the script fails (`raw.githubusercontent.com` blocked), fetch it
from a CDN mirror instead. Once the script is running, it switches to mirrors for
its own downloads on its own:

```sh
curl -fsSL https://cdn.jsdelivr.net/gh/chenchaoyi/gtmux@main/install.sh | bash
```

Two mirror chains are involved, one for each kind of download:

- The install script itself. `gtmux update` fetches it from GitHub first, then
  tries jsdelivr, gh-proxy.com, ghfast.top and ghproxy.net in that order. So once
  gtmux is installed, updates work on a mainland network without any of this.
- The release files (the CLI tarball and the app zip). The installer tries GitHub
  first and, when a download stalls, tries ghfast.top, gh-proxy.com and ghproxy.net
  in that order. `SHASUMS256.txt` is always fetched from GitHub first, so the
  checksum stays anchored on GitHub even when the tarball came through a mirror.

Override with `GTMUX_INSTALL_MIRROR`:

```sh
GTMUX_INSTALL_MIRROR=ghproxy  curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash   # straight to the mirror chain
GTMUX_INSTALL_MIRROR=https://my.mirror/  curl -fsSL ... | bash   # custom <prefix><github-url> proxy
GTMUX_INSTALL_MIRROR=github   curl -fsSL ... | bash   # GitHub only, no mirrors
```

## Moving to a new Mac

The one thing worth carrying is HQ's records: the notes HQ keeps about your
sessions, the knowledge it has collected, and `LOCAL.md`, the file that holds your
own preferences. gtmux does not regenerate any of it.

```sh
# on the old Mac
gtmux hq --export ~/gtmux-hq.tar.gz

# on the new one, after installing gtmux
gtmux hq --import ~/gtmux-hq.tar.gz
gtmux hq                      # restart HQ so it reads the restored notes
```

The export asks you for a passphrase and locks the file with it (`--plain` skips
the lock); the import asks for the same passphrase. An import never overwrites in
place: anything already there is moved to `hq.replaced-<timestamp>` and the path
is printed.

Everything else is quicker to re-create than to copy. Run `gtmux doctor --fix` on
the new machine: it installs the agent hooks, set-titles, restore-after-reboot and
the menu-bar app. Pair the phone again (`gtmux pair`, or `gtmux tunnel`, which
prints a pairing QR) instead of copying pairing files, so the tokens the old Mac
issued stop being valid.

`~/.local/share/gtmux/` holds live state (markers, events, snapshots). Leave it behind.

## Signing & permissions

macOS ties the permissions you grant to the app's code signature. Official
releases are signed with a Developer ID and notarized in CI, so your grants
survive updates. A build you make yourself with `make app` is ad-hoc signed; its
identity changes with every rebuild, so macOS forgets the grants and asks again.
To sign your own build with a Developer ID, set `GTMUX_SIGN_ID` when building
(see `macapp/build.sh`).
