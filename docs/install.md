# Install

**English** · [中文](install.zh.md)

## Agent plugins and hooks

After installing the CLI/app, `gtmux install hooks --agent auto` detects supported
agent homes/installations and skips hooks disabled by settings. Use `--agent dsh`
for DSH alone; omitting `--agent` keeps the Claude default. `gtmux doctor` inspects;
`gtmux doctor --fix` offers repairs one by one. The DSH Cordis plugin is bundled,
with no separate npm package. History reading needs no plugin; lifecycle events
require the generated `--patch` on the next Web launch. See the [DSH guide](guides/deepseek-harness.md).

## Linux

The same installer supports Linux amd64/arm64 and automatically skips the macOS app. Install tmux with your distribution's package manager. See [Linux server](linux.md) for pairing, foreground access, systemd user services and platform limits.

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
installs as a Homebrew cask. The app cask defaults to `/Applications`; a custom
Homebrew `--appdir` changes that destination. Without Homebrew, use the install
script below.

## Install script

```sh
curl -fsSL https://ccy.dev/gtmux/install | bash
```

Installs the checksum-verified binary to `~/.local/bin/gtmux`, plus the menu-bar
app on macOS. Options:

- `GTMUX_NO_APP=1`: skip the menu-bar app (CLI only).
- `GTMUX_APP_LOGIN=1`: start the app at login.
- `GTMUX_VERSION=vX.Y.Z`: pin a version.

Put these variables on `bash`, which runs the installer, rather than on `curl`:

```sh
curl -fsSL https://ccy.dev/gtmux/install | GTMUX_NO_APP=1 bash
```

From source (Go 1.26 or newer; CLI only):

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

An ordinary source build does not include the official hosted-tunnel registration
or push-relay credentials. Use an official release for those services, or configure
your own services using [the tunnel design](design/remote-access-tunnel.md) and
[the push-relay reference](../relay/README.md).

Update later with `gtmux update`; it also attempts to restart the loaded serve and
Direct tunnel services so they use the new binary.

Automatic updates preserve components already newer than the selected release and
leave unknown versions alone. Cached version indexes may lag GitHub. The existing
`GTMUX_VERSION` pin explicitly selects a version, including a downgrade.
If the optional app fails after CLI installation, the CLI stays installed and exit 3
reports the remaining app step; retry `gtmux update` for an older app or `gtmux install app` if absent. See the [update contract](cli.md#installation-and-update-commands).


For a script-installed app in `~/Applications`, `gtmux uninstall app` removes the
menu-bar app and its login item. `gtmux uninstall hooks` unregisters the agent
hooks, and `gtmux uninstall all` does both. These commands leave the CLI binary
and gtmux's saved records in place. For Homebrew installations, also use
`brew uninstall --cask gtmux-app` to remove the app (normally in `/Applications`), and
`brew uninstall --cask gtmux` to remove the CLI.

## Official mirrors and GitHub fallback

Both official website entries are available: `https://ccy.dev/gtmux/install` and `https://ccy.pub/gtmux/install`. The short bootstrap authenticates metadata from the owned download origins, then verifies the installer and every archive. Invalid signatures stop rather than weaken verification. Native updater builds containing this implementation check the same public key in Go and never execute a remote script; existing binaries keep their original update channel until upgraded. Network unavailability retains the existing GitHub channel; once signed metadata is available, fallback bytes must still match its digest and size.

See [release hosting](release-hosting.md) for build, storage, source and operator boundaries. The short bootstrap requires OpenSSL/LibreSSL; macOS includes LibreSSL. Older clients, Homebrew and explicit overrides keep their existing channels. To force GitHub or a third-party proxy, fetch the full installer from GitHub:

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=ghproxy bash
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=https://my.mirror/ bash
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=github bash
```

`ghproxy` starts release-archive downloads with the mirror chain; checksum files
still try GitHub first. Replace `https://my.mirror/` with your proxy prefix; this
mode tries GitHub, then `<prefix><github-url>`. `github` disables mirror fallback.
These options affect downloads made by the installer, not the preceding `curl`.

## Moving to a new Mac

The one thing worth carrying is HQ's records: the notes HQ keeps about your
sessions, the knowledge it has collected, and `LOCAL.md`, the file that holds your
own preferences. gtmux does not regenerate any of it.

```sh
# on the old Mac
gtmux hq --export ~/gtmux-hq.tar.gz.age

# on the new one, after installing gtmux and exiting any running HQ agent
gtmux hq --import ~/gtmux-hq.tar.gz.age
gtmux hq                      # restart HQ so it reads the restored notes
```

The export asks you for a passphrase and locks the file with it (`--plain` skips
the lock); the import asks for the same passphrase. An import never overwrites in
place: anything already there is moved aside to a sibling folder named
`hq.replaced-…`, and the path is printed.

Run `gtmux doctor --fix` in an interactive terminal on the new machine. It offers
the missing agent hooks, set-titles, restore-after-reboot and menu-bar app setup,
explains each change and asks before applying it. Pair the phone again (`gtmux pair`, or `gtmux tunnel`, which
prints a pairing QR) instead of copying pairing files, so the tokens the old Mac
issued are not accepted by the new Mac. This does not revoke access to the old
Mac; revoke its devices there if you are retiring it.

Do not copy `~/.local/share/gtmux/` wholesale: pane markers refer to the old Mac's
panes, while `hq-snapshots/` holds HQ-home archives that overlap your HQ export.
The state directory also holds local event and usage history; an HQ export does
not carry those histories to the new Mac.

## Signing & permissions

macOS ties the permissions you grant to the app's code signature. Official
releases are signed with a Developer ID and notarized in CI, so your grants
survive updates. A build you make yourself with `make app` is ad-hoc signed; its
identity changes with every rebuild, so macOS forgets the grants and asks again.
To sign your own build with a Developer ID, set `GTMUX_SIGN_ID` when building
(see `macapp/build.sh`).
