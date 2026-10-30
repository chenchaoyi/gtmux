# Install, update and uninstall

**English** · [中文](install.zh.md)

For the shortest path from installation to a connected device, start with [Get connected](../README.md#get-connected). This page covers installation choices, doctor results and maintenance.

## Choose a download

| What you need | Install |
| --- | --- |
| CLI and menu-bar app on a Mac | The official installer below |
| Manage both through Homebrew | The two Homebrew commands below |
| iPhone or iPad app | [App Store](https://apps.apple.com/app/id6791144062); pair it after setting up the host |
| CLI only or a specific release | Installer options below, or [GitHub Releases](https://github.com/chenchaoyi/gtmux/releases) |

## Install script

```sh
curl -fsSL https://ccy.dev/gtmux/install | bash
```

Use `https://ccy.pub/gtmux/install` in mainland China. Both official entries verify signed release metadata and the downloaded installer and packages. Invalid signatures stop installation. GitHub retains the original release archives.

The script installs the CLI to `~/.local/bin/gtmux` and, on macOS, installs and opens the menu-bar app in `~/Applications` (or updates its existing `/Applications` installation). It does not install tmux: on a Mac, use `brew install tmux` if needed. Install and sign in to the coding-agent CLI you want to use too.

If the shell says `gtmux: command not found`, add this line to your shell configuration, as the installer instructs, then open a new terminal:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

Installer options go on `bash`, which runs the script:

```sh
curl -fsSL https://ccy.dev/gtmux/install | GTMUX_NO_APP=1 bash
curl -fsSL https://ccy.dev/gtmux/install | GTMUX_VERSION=vX.Y.Z bash
```

Replace `vX.Y.Z` with a published tag. `GTMUX_NO_APP=1` installs only the CLI; `GTMUX_APP_LOGIN=1` also enables the app's login item. For manual downloads, choose the archive matching your OS and architecture; `Gtmux-<version>-macos.zip` is the Mac app, not the CLI archive.

## Configure with doctor

```sh
gtmux doctor
```

The report groups checks by the feature they affect. In an interactive terminal, it offers **Review available repairs now?** when something needs attention. Accept to review each change, or start that review directly:

```sh
gtmux doctor --fix
```

Doctor can configure tmux titles and pane labels, scrollback and session restoration, snapshot plugins and autosave, and the supported hooks for agents installed on this machine. It also offers supported agent settings and missing app/dependency repairs. It explains each action before asking; separate system permissions or manual work remain as guidance. Existing settings outside gtmux's managed blocks are preserved, and tmux config edits get a `.gtmux.bak` backup.

If tmux is absent, install it first. If its server is not running, create a session with `tmux new -s work` and retry from there. A socket permission error needs to be resolved before repairs can apply; do not run gtmux as a different user to work around it.

After repairs, run `gtmux doctor` again. **✗** identifies a condition blocking a feature, **⚠** a recommendation, **✓** a passing check and **·** information. Read the remaining checks and their suggested actions; “applied fixes” can still have recommendations left. Hooks marked **Configured** describe saved settings. Let each agent load them on its next normal launch; a running session is not restarted.

For a read-only report, use `gtmux doctor --no-prompt` or `gtmux doctor --json`. The Mac app also exposes **Preferences → General → Environment check**. If a problem persists, inspect `gtmux logs --since 1h`; `gtmux doctor --bundle` writes a local diagnostic archive for review before you share it.

For an intentional per-agent setup, use `gtmux install hooks --agent codex` (replace the key), or `--agent auto` for installed supported CLIs. Omitting `--agent` selects Claude Code. [Agent setup](guides/agent-integrations.md) describes the supported keys and loading requirements; DSH's bundled Cordis module needs its printed overlay on the next Web launch.

## Homebrew (macOS)

```sh
brew install chenchaoyi/tap/gtmux
brew install --cask chenchaoyi/tap/gtmux-app
```

The CLI is distributed as a Homebrew cask. The app normally goes to `/Applications`; Homebrew's `--appdir` can change that. Upgrade with `brew upgrade gtmux` and `brew upgrade --cask gtmux-app`.

<a id="platform-support"></a>

## Platform support and source builds

The same installer provides the cgo-free CLI/server for macOS and Linux amd64/arm64, skipping the app on Linux. Install tmux with your system package manager and run gtmux as the same user as the tmux sessions. Desktop focus, the menu-bar app and lid controls are macOS features. Linux release archives remain available on [GitHub Releases](https://github.com/chenchaoyi/gtmux/releases).

A source build needs a patched stable Go toolchain (1.26.10 / 1.27.3 or newer):

```sh
go install github.com/chenchaoyi/gtmux/cmd/gtmux@latest
```

Source builds install only the CLI and do not contain the official hosted-tunnel registration or push-relay credentials. Use an official release for those services. Self-hosting details belong in the [tunnel design](design/remote-access-tunnel.md) and [relay reference](../relay/README.md).

## Update or uninstall

`gtmux update` updates the CLI and menu-bar app and attempts to restart loaded serve and Direct tunnel services with the new binary. Higher component versions are preserved; unknown versions are left alone. A `GTMUX_VERSION` pin can explicitly select a version, including a downgrade. See the [update contract](cli.md#installation-and-update-commands).

Exit 3 after a CLI installation means the optional app step failed. For an old app retry `gtmux update`; for an absent one use `gtmux install app`.

`gtmux uninstall app` removes a script-installed app and its login item; `gtmux uninstall hooks` removes gtmux's agent hooks. `gtmux uninstall all` does both, preserving the CLI binary and saved records. For Homebrew installations, use `brew uninstall --cask gtmux-app` and `brew uninstall --cask gtmux` to remove those components.

## Download problems

Try the other official installer domain first. The short bootstrap requires OpenSSL/LibreSSL; macOS includes LibreSSL. If you intentionally need the original GitHub installer or a proxy, use:

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=github bash
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | GTMUX_INSTALL_MIRROR=ghproxy bash
```

`github` disables mirror fallback. `ghproxy` uses the mirror chain for release archives; checksum files still try GitHub first. These options affect the installer downloads, not the initial `curl`. Signed metadata, when available, still binds fallback bytes to its digest and size. [Release hosting](release-hosting.md) covers the verification and operator boundaries.

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

## Standard tunnel security updates

The optional `cloudflared` client must meet Cloudflare's certificate security minimum:
2026.10.2 or later, or 2026.10.3 or later for non-Homebrew macOS installations.
Unknown and prerelease versions cannot start a new Standard or quick tunnel.
Direct uses gtmux's own transport and does not use this executable.

`gtmux install cloudflared --yes` installs or updates this client explicitly.
On macOS, gtmux upgrades an existing Homebrew client through the Homebrew prefix
that owns it. Other installation channels and Linux package managers require their
own update command; gtmux explains the remaining action and returns failure.
It never disables TLS verification or switches installation channels automatically.

The install script, `gtmux install all`, and normal `gtmux update` maintain an
already-installed client. They do not install an absent optional client.
`update --check` / `--json` stay read-only; `--cli-only` skips dependency updates.
Older gtmux updaters cannot acquire this behavior retroactively: after upgrading
from a release without this integration, run `gtmux install cloudflared --yes` once.
Homebrew's gtmux cask update alone does not run this dependency repair.

Replacing the executable does not update an existing process. `doctor` distinguishes
installed and running versions using the managed connector's loopback metrics.
An old or unverifiable running Standard connector keeps the update incomplete;
run `gtmux tunnel --service --recover`, then check the connection. Restarting drops
existing long-lived connections. When its installation channel is unknown, the
running Mac check uses the stricter 2026.10.3 minimum. Legacy login services and
connectors on other hosts still need checking; this is not an account-wide audit.

Official gtmux CLI and bundled-CLI builds require a stable Go toolchain containing
the GO-2026-6809 fix (Go 1.27.3 / 1.26.10 or newer). Use one for custom source builds
too. Keep OS trust stores current and replace affected HTTPS origin certificates;
do not use `noTLSVerify` to bypass errors. See the
[Cloudflare incident](https://blog.cloudflare.com/emudhra-certificate-compromise/),
[update guide](https://developers.cloudflare.com/tunnel/guides/update-cloudflared/)
and [Go advisory](https://pkg.go.dev/vuln/GO-2026-6809).
