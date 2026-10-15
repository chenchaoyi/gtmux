# Linux server

**English** · [中文](linux.zh.md)

Run gtmux on the Linux host that owns your tmux sessions. The terminal, browser, iPhone and iPad use that host's state. A Mac menu-bar app still manages its local host; this does not merge multiple hosts into one HQ.

## Install and connect

Linux amd64 and arm64 releases are cgo-free. Ubuntu 22.04+/Debian 12+ are the initial target environments. Install `tmux`, `curl`, `tar` and a SHA256 tool with your distribution's package manager, then:

```sh
curl -fsSL https://raw.githubusercontent.com/chenchaoyi/gtmux/main/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"
gtmux doctor --no-prompt
gtmux install hooks --agent codex --yes
# Or --agent claude, cursor, gemini, kimi, copilot, kiro.
```

The installer chooses `linux-amd64` or `linux-arm64`, verifies the checksum and skips the macOS app. Restart an agent to load new hooks. Install each agent separately using its own instructions. A source build needs Go 1.26+ and does not include official hosted-service credentials.

Run gtmux as the **same user** as tmux. Use the same socket/environment when using a nondefault tmux server. Do not use sudo to search another user's sessions.

```sh
gtmux serve --bind 127.0.0.1 --port 8765
```

For another device on a trusted network, bind the intended interface (for example `--bind 0.0.0.0`), then run `gtmux pair` in another shell and scan its code in the app. Keep direct listeners behind a firewall/VPN; the owner token authorizes terminal input. For remote access through a tunnel:

```sh
gtmux tunnel                        # Standard; install cloudflared first
gtmux tunnel --backend self         # Direct; requires an existing access grant/config
```

[Install cloudflared](https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/) from the official source. gtmux does not run sudo or install distribution packages automatically. Direct uses its embedded tunnel client and does not need cloudflared. Foreground commands need no systemd. Existing enrollment, grants, pairing and token rules are unchanged.

## Background operation

```sh
gtmux serve --service               # Local-network access
# Or, explicitly enable one tunnel backend:
gtmux tunnel --service --yes
gtmux tunnel --backend self --service --yes
gtmux tunnel --status
gtmux tunnel --unservice            # Stop/remove gtmux remote-access services
```

Linux uses **systemd user units**: `com.gtmux.serve.service`, plus `com.gtmux.tunnel.service` (Standard) or `com.gtmux.selftunnel.service` (Direct). Files go in `$XDG_CONFIG_HOME/systemd/user`, or `~/.config/systemd/user`. They are private (0600). Setup checks the user manager before changing remote access. An unavailable user bus is a setup error, not a successful install: use a proper login session or foreground mode.

User services normally start with the user's manager. To keep them running after logout and start the manager at boot, an administrator may need to enable lingering for that account. gtmux **does not enable it**. Review [systemd's loginctl guidance](https://www.freedesktop.org/software/systemd/man/252/loginctl.html) and local policy before doing so. Enabling a unit alone is not proof of reboot/logout persistence or Internet reachability.

The unit preserves HOME, the installer caller's PATH and an explicitly selected tmux socket. It does not copy the entire environment or credentials. If an agent uses a custom configuration directory, arrange its environment in your account/service configuration. Restarting serve updates remote assets without terminating tmux sessions it created (`KillMode=process` is intentional). Stopping remote access leaves local agent work intact.

```sh
systemctl --user status com.gtmux.serve.service
journalctl --user -u com.gtmux.serve.service --since today
gtmux logs
gtmux doctor --no-prompt
gtmux update
```

Application captures remain under `~/.local/share/gtmux/logs/`; activity/event/knowledge records use the same paths and formats as macOS. `gtmux update` restarts active gtmux services; a restart failure is reported and needs a manual retry. Unit edits outside gtmux are not silently overwritten by automatic Standard connector recovery.

## Environment checks and repairs

`gtmux doctor` checks tmux connectivity and settings, hooks for agents with existing configuration, storage/logs, remote access and installed systemd user units. Desktop automation, menu-bar installation, macOS sleep settings and tab titles required only for desktop focus are not Linux repairs. Foreground use does not require a systemd user manager. An installed but inactive unit is reported with its `systemctl --user status` command.

Run `gtmux doctor --fix` in the current terminal to review reversible repairs. `--yes` applies those repairs without per-step prompts; it does not install system packages, enable lingering, start remote access or create settings for an unused Claude installation. Locale repair uses `C.UTF-8`, updates the tmux server and its managed configuration, and leaves explicit `LC_ALL`/`LC_CTYPE` overrides to the user. Existing panes retain their environment. JSON reports are complete even when checks find problems (exit 1); recommendations alone return 0.

## Available features and limits

- tmux radar, terminal/chat history, input, remote new sessions, dispatch/spawn, restore, HQ and knowledge use the shared core. Headless restore leaves sessions available and prints `tmux attach` instructions. `gtmux focus` switches an interactive local tmux client; without one it gives attach guidance and returns 2. Remote desktop focus remains unsupported.
- Phone push and Live Activities use the configured push relay, independently of desktop banners. Official releases include the release relay configuration; source builds need a relay setup. Existing app permissions and push limits apply.
- The macOS menu bar, desktop window focus, screen annotation and lid/power controls are not Linux features. Clients use host capability facts and omit desktop-only actions where available. Old clients receive an honest unsupported response.
- Resources: disk is the filesystem containing gtmux state; memory uses Linux `MemAvailable / MemTotal` (warn below 10%, critical below 5%); load is 1-minute load divided by accessible CPUs. Battery is not monitored in this Linux server implementation. Missing applicable readings cannot clear an alarm. Container/cgroup-specific pressure is not measured.
- HQ supervises one host. Native sessions outside tmux retain their existing sensing/adoption limitations. Real agent accounts, mobile push delivery, iPhone/iPad layout and Linux login/logout persistence need environment-specific acceptance; CI fixtures do not establish them.
- Other Unix systems, including FreeBSD, are not yet supported releases. Platform adapters provide the extension points without promising current support.
