package app

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/userservice"
)

// Always-on remote access (explicit opt-in). `gtmux tunnel --service` registers
// two launchd LaunchAgents so the Mac stays reachable across reboots WITHOUT
// re-running the command: one runs `gtmux serve` (on loopback: the radar, panes
// and, for an owner, terminal input), one runs `cloudflared` with the provisioned connector token. This is
// a STANDING exposure (token-gated), so it is never a default — it's opt-in, and
// `--unservice` removes it. The menu-bar app surfaces an on/off toggle + a visible
// indicator so it's never silent.

const (
	serveAgentLabel  = "com.gtmux.serve"
	tunnelAgentLabel = "com.gtmux.tunnel"
)

var serviceManager = func() userservice.Manager { return userservice.Current(homeDir()) }

func userServicesDir() string { return serviceManager().Dir() }
func serveAgentPath() string  { return serviceManager().Path(serveAgentLabel) }
func tunnelAgentPath() string { return serviceManager().Path(tunnelAgentLabel) }
func writeUserService(path, label string, args []string, log string) error {
	return serviceManager().Write(path, label, args, log)
}
func serviceLoaded(label string) bool { return serviceManager().Active(label) }
func userService(action, path string) error {
	switch action {
	case "load":
		return serviceManager().Start(path)
	case "unload":
		if !fileExists(path) {
			return nil
		}
		return serviceManager().Stop(path)
	default:
		return fmt.Errorf("unsupported service action %s", action)
	}
}
func printServicePersistence() {
	if serviceManager().NeedsLinger() {
		i18n.Say("User services start with your user manager. For logout/reboot persistence, review loginctl enable-linger; gtmux does not enable it.", "服务随用户管理器启动。若需退出登录或重启后常驻，请先了解 loginctl enable-linger；gtmux 不会自动开启。")
	} else {
		i18n.Say("Services start at login.", "服务在登录后启动。")
	}
}

func stopUserServices(paths ...string) error {
	for _, path := range paths {
		if !fileExists(path) {
			continue
		}
		if err := userService("unload", path); err != nil {
			return err
		}
	}
	return nil
}
func removeUserServices(paths ...string) error {
	if err := stopUserServices(paths...); err != nil {
		return err
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
}

func preflightUserServices() bool {
	if err := serviceManager().Check(); err != nil {
		i18n.Sae(err.Error(), "后台服务管理器不可用："+err.Error())
		return false
	}
	return true
}

// tunnelURLPath stores the stable URL of the always-on tunnel (for --status and
// the menu-bar indicator).
func tunnelURLPath() string {
	return state.TunnelURLPath()
}

func serviceInstalled() bool {
	return fileExists(serveAgentPath()) && tunnelBackend() != "none"
}

// tunnelServiceInstall provisions the stable tunnel and registers the always-on
// launchd agents (after explaining the standing exposure and confirming).
func tunnelServiceInstall(port int, name string, yes, force, recover bool) int {
	if !preflightUserServices() {
		return 1
	}
	reg := tunnelRegSecret()
	if reg == "" {
		i18n.Sae("gtmux tunnel: always-on needs hosted mode (not configured in this build).",
			"gtmux tunnel: always-on 需要托管模式（此构建未启用）。")
		return 2
	}
	// lookTool, not exec.LookPath: the menu-bar app shells out with launchd's PATH,
	// where /usr/local/bin isn't listed — cloudflared read as "not installed" on a Mac
	// that had it, and Anywhere could never be switched on from the menu bar.
	bin := lookTool("cloudflared")
	if bin == "" {
		if bin = ensureCloudflared(yes); bin == "" {
			return 1
		}
	}

	// --yes bypasses the prompt (the menu-bar toggle shows its own confirmation).
	if !yes {
		i18n.Say(i18n.Bold+"Enable background remote access?"+i18n.Reset,
			i18n.Bold+"开启后台远程访问？"+i18n.Reset)
		i18n.Say(i18n.Dim+"  This registers two background services (the tunnel client + gtmux serve) that start"+i18n.Reset,
			i18n.Dim+"  这会注册两个后台服务（隧道客户端 + gtmux serve），随用户服务启动，"+i18n.Reset)
		i18n.Say(i18n.Dim+"  at login and keep your computer reachable at a public URL (token-gated) until you"+i18n.Reset,
			i18n.Dim+"  让你的计算机持续在一个公网地址可达（有 token 把关），直到你跑"+i18n.Reset)
		i18n.Say(i18n.Dim+"  run `gtmux tunnel --unservice`. It is a standing exposure, so enable it deliberately."+i18n.Reset,
			i18n.Dim+"  `gtmux tunnel --unservice` 关闭。这是个长期敞口，请有意识地开启。"+i18n.Reset)
		if !confirmRisky(i18n.Tr("  enable always-on? [y/N] ", "  开启 always-on？[y/N] ")) {
			i18n.Say("  skipped.", "  已跳过。")
			return 0
		}
	}

	if recover {
		i18n.Say("Restoring your Standard connection…", "正在恢复 Standard 连接…")
	} else if force {
		i18n.Say("Repairing your existing Standard tunnel address…", "正在修复现有 Standard 隧道地址…")
	} else {
		i18n.Say("Requesting your stable tunnel address…", "正在申请你的固定隧道地址…")
	}
	deviceID := ""
	if force || recover {
		b, err := os.ReadFile(filepath.Join(state.ConfigDir(), "tunnel-device-id"))
		if err != nil || len(bytes.TrimSpace(b)) < 16 {
			i18n.Sae("No saved Standard device identity to recover. Enable remote access first.", "没有可恢复的 Standard 设备身份，请先开启远程访问。")
			return 1
		}
		deviceID = string(bytes.TrimSpace(b))
	} else {
		deviceID = resolveDeviceID()
	}
	prov, err := provisionTunnel(tunnelAPI(), reg, deviceID, name, force, recover)
	if err != nil {
		if recover {
			if errors.Is(err, errTunnelRecoveryUnsupported) {
				i18n.Sae("Connection recovery is not available yet. Update gtmux or try again later.",
					"连接恢复功能暂不可用。请更新 gtmux 或稍后重试。")
			} else {
				i18n.Sae("Couldn't restore the connection. Previous settings are kept; check your network or proxy, or try again later.",
					"暂时无法恢复连接，原配置已保留。请检查网络或代理线路，或稍后重试。")
			}
			tunnelDebugf("recovery failed: %v", err)
		} else {
			i18n.Sae("gtmux tunnel: provision failed: "+err.Error(), "gtmux tunnel: 申请失败："+err.Error())
		}
		return 1
	}

	unlock, err := lockRemoteServices()
	if err != nil {
		i18n.Sae("Remote access is being updated; try again shortly.", "远程访问正在更新，请稍后重试。")
		return 1
	}
	defer unlock()
	if recover && readTunnelURL() != "" && prov.URL != readTunnelURL() {
		i18n.Sae("Recovery did not preserve your address. Previous settings are kept; update the tunnel service.", "恢复未能保留原地址，原配置已保留；请更新隧道服务。")
		return 1
	}
	token := resolveServeToken("")

	_ = os.MkdirAll(state.LogsDir(), 0o700) // launchd will not create the capture's directory
	if err := os.MkdirAll(userServicesDir(), 0o755); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}

	// serve on loopback (the tunnel reaches it locally; nothing extra on the LAN).
	if err := writeUserService(serveAgentPath(), serveAgentLabel,
		[]string{selfPath(), "serve", "--bind", "127.0.0.1", "--port", strconv.Itoa(port)},
		state.CapturePath("serve")); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	// cloudflared with the connector token (0600 — the plist holds the token).
	if err := writeUserService(tunnelAgentPath(), tunnelAgentLabel,
		[]string{bin, "tunnel", "--metrics", cloudflaredMetricsAddr, "run", "--protocol", cloudflaredProtocol(), "--token", prov.Token},
		state.CapturePath("cloudflared")); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	writeTunnelURL(prov.URL)

	// Backends are mutually exclusive — retire a self-hosted tunnel agent if present.
	if fileExists(selfTunnelAgentPath()) {
		if err := removeUserServices(selfTunnelAgentPath()); err != nil {
			i18n.Sae(err.Error(), err.Error())
			return 1
		}
	}
	if err := stopUserServices(serveAgentPath(), tunnelAgentPath()); err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 1
	}
	if err := userService("load", serveAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: start serve service: "+err.Error(), "gtmux tunnel: start serve service: "+err.Error())
		return 1
	}
	if err := userService("load", tunnelAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: start tunnel service: "+err.Error(), "gtmux tunnel: start tunnel service: "+err.Error())
		return 1
	}

	printPairingBlock(prov.URL, token, name, port)
	i18n.Say(i18n.Dim+"Always-on enabled: background services are running. Turn off: `gtmux tunnel --unservice`."+i18n.Reset,
		i18n.Dim+"后台服务已启动。关闭：`gtmux tunnel --unservice`。"+i18n.Reset)
	printServicePersistence()
	return 0
}

// serveServiceInstall registers a LAN-only serve LaunchAgent (bind 0.0.0.0 so the
// phone reaches it on the same local network) that survives reboots. This is the free
// local-network remote mode; it removes any always-on tunnel agent first, since
// LAN and anywhere are mutually exclusive remote-access modes (the menu-bar app
// exposes them as one Off / Local network / Anywhere chooser).
func serveServiceInstall(port int) int {
	if !preflightUserServices() {
		return 1
	}
	unlock, err := lockRemoteServices()
	if err != nil {
		i18n.Sae("Remote access is being updated; try again shortly.", "远程访问正在更新，请稍后重试。")
		return 1
	}
	defer unlock()

	_ = resolveServeToken("") // ensure the persistent serve-token exists (0600)

	// drop the tunnel layer if present — this mode is LAN-only. BOTH backends:
	// com.gtmux.tunnel (Cloudflare) AND com.gtmux.selftunnel (Direct), else switching
	// the local network while on Direct leaves the self-tunnel up and the mode reads .anywhere.
	if err := removeUserServices(tunnelAgentPath(), selfTunnelAgentPath()); err != nil {
		i18n.Sae("Could not stop the previous tunnel; remote mode was not changed.", "未能停止原隧道，未切换访问模式。")
		return 1
	}
	removeTunnelURL()

	_ = os.MkdirAll(state.LogsDir(), 0o700) // launchd will not create the capture's directory
	if err := os.MkdirAll(userServicesDir(), 0o755); err != nil {
		i18n.Sae("gtmux serve: "+err.Error(), "gtmux serve: "+err.Error())
		return 1
	}
	if err := writeUserService(serveAgentPath(), serveAgentLabel,
		[]string{selfPath(), "serve", "--bind", "0.0.0.0", "--port", strconv.Itoa(port)},
		state.CapturePath("serve")); err != nil {
		i18n.Sae("gtmux serve: "+err.Error(), "gtmux serve: "+err.Error())
		return 1
	}
	if err := stopUserServices(serveAgentPath()); err != nil {
		i18n.Sae(err.Error(), err.Error())
		return 1
	}
	if err := userService("load", serveAgentPath()); err != nil {
		i18n.Sae("gtmux serve: start service: "+err.Error(), "gtmux serve: start service: "+err.Error())
		return 1
	}
	i18n.Say("Local-network access enabled. Turn off: `gtmux serve --unservice`.",
		"局域网访问已开启。关闭：`gtmux serve --unservice`。")
	printServicePersistence()
	return 0
}

// serviceRemoveAll unloads + removes whichever remote-access agents exist (the
// LAN serve and/or the always-on tunnel). It backs both `gtmux serve --unservice`
// and the menu-bar "Off" choice, so turning remote access off works from any mode.
func serviceRemoveAll() int {
	unlock, err := lockRemoteServices()
	if err != nil {
		i18n.Sae("Remote access is being updated; try again shortly.", "远程访问正在更新，请稍后重试。")
		return 1
	}
	defer unlock()
	if err := removeUserServices(serveAgentPath(), tunnelAgentPath(), selfTunnelAgentPath()); err != nil {
		i18n.Sae("Could not stop all remote services. Settings were retained; retry after checking the service manager.", "未能停止全部远程服务，配置已保留。请检查服务管理器后重试。")
		return 1
	}
	removeTunnelURL()
	i18n.Say("Remote access disabled: background services stopped and removed.", "远程访问已关闭，后台服务已停止并移除。")
	return 0
}

func tunnelServiceRemove() int { return serviceRemoveAll() }

// tunnelServiceStatus reports whether always-on is active and at which URL.
func tunnelServiceStatus() int {
	if !serviceInstalled() {
		i18n.Say("Always-on: off  (run `gtmux tunnel --service` to enable, or `gtmux tunnel` for a foreground session)",
			"Always-on：关闭  （跑 `gtmux tunnel --service` 开启，或 `gtmux tunnel` 前台开一次）")
		return 0
	}
	backend := tunnelBackend()
	label := tunnelAgentLabel
	if backend == "direct" {
		label = selfTunnelAgentLabel
	}
	loaded := serviceLoaded(serveAgentLabel) && serviceLoaded(label)
	state := i18n.Tr("on", "开启")
	if !loaded {
		state = i18n.Tr("installed but not running (try re-login or --service again)",
			"已安装但未运行（重新登录或再跑 --service）")
	}
	i18n.Say("Always-on: "+state, "Always-on:"+state)
	if backend == "direct" {
		i18n.Say("  Tunnel: Direct", "  隧道：直连")
	} else {
		i18n.Say("  Tunnel: Standard", "  隧道：标准")
	}
	if u := readTunnelURL(); u != "" {
		fmt.Printf("  URL: %s\n", u)
	}
	return 0
}
