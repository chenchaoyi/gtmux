package app

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Always-on remote access (explicit opt-in). `gtmux tunnel --service` registers
// two launchd LaunchAgents so the Mac stays reachable across reboots WITHOUT
// re-running the command: one runs `gtmux serve` (the read-only radar on
// loopback), one runs `cloudflared` with the provisioned connector token. This is
// a STANDING exposure (token-gated), so it is never a default — it's opt-in, and
// `--unservice` removes it. The menu-bar app surfaces an on/off toggle + a visible
// indicator so it's never silent.

const (
	serveAgentLabel  = "com.gtmux.serve"
	tunnelAgentLabel = "com.gtmux.tunnel"
)

func launchAgentsDir() string { return filepath.Join(homeDir(), "Library", "LaunchAgents") }
func serveAgentPath() string  { return filepath.Join(launchAgentsDir(), serveAgentLabel+".plist") }
func tunnelAgentPath() string { return filepath.Join(launchAgentsDir(), tunnelAgentLabel+".plist") }

// tunnelURLPath stores the stable URL of the always-on tunnel (for --status and
// the menu-bar indicator).
func tunnelURLPath() string {
	return filepath.Join(homeDir(), ".config", "gtmux", "tunnel-url")
}

func serviceInstalled() bool {
	return fileExists(serveAgentPath()) && fileExists(tunnelAgentPath())
}

// tunnelServiceInstall provisions the stable tunnel and registers the always-on
// launchd agents (after explaining the standing exposure and confirming).
func tunnelServiceInstall(port int, name string, yes bool) int {
	reg := tunnelRegSecret()
	if reg == "" {
		i18n.Sae("gtmux tunnel: always-on needs hosted mode (not configured in this build).",
			"gtmux tunnel: always-on 需要托管模式（此构建未启用）。")
		return 2
	}
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		if bin = ensureCloudflared(); bin == "" {
			return 1
		}
	}

	// --yes bypasses the prompt (the menu-bar toggle shows its own confirmation).
	if !yes {
		i18n.Say(i18n.Bold+"Keep remote access ON across reboots?"+i18n.Reset,
			i18n.Bold+"让远程访问重启后也保持开启？"+i18n.Reset)
		i18n.Say(i18n.Dim+"  This registers two background services (the tunnel client + gtmux serve) that start"+i18n.Reset,
			i18n.Dim+"  这会注册两个后台服务（隧道客户端 + gtmux serve），开机自启、"+i18n.Reset)
		i18n.Say(i18n.Dim+"  at login and keep your Mac reachable at a public URL (token-gated) until you"+i18n.Reset,
			i18n.Dim+"  让你的 Mac 持续在一个公网地址可达（有 token 把关），直到你跑"+i18n.Reset)
		i18n.Say(i18n.Dim+"  run `gtmux tunnel --unservice`. It is a standing exposure — enable consciously."+i18n.Reset,
			i18n.Dim+"  `gtmux tunnel --unservice` 关闭。这是个长期敞口，请有意识地开启。"+i18n.Reset)
		if !confirmRisky(i18n.Tr("  enable always-on? [y/N] ", "  开启 always-on？[y/N] ")) {
			i18n.Say("  skipped.", "  已跳过。")
			return 0
		}
	}

	i18n.Say("Requesting your stable tunnel address…", "正在申请你的固定隧道地址…")
	prov, err := provisionTunnel(tunnelAPI(), reg, resolveDeviceID(), name)
	if err != nil {
		i18n.Sae("gtmux tunnel: provision failed: "+err.Error(), "gtmux tunnel: 申请失败："+err.Error())
		return 1
	}
	token := resolveServeToken("")

	logDir := filepath.Join(homeDir(), ".local", "share", "gtmux")
	_ = os.MkdirAll(logDir, 0o755)
	if err := os.MkdirAll(launchAgentsDir(), 0o755); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}

	// serve on loopback (the tunnel reaches it locally; nothing extra on the LAN).
	if err := writeLaunchAgent(serveAgentPath(), serveAgentLabel,
		[]string{selfPath(), "serve", "--bind", "127.0.0.1", "--port", strconv.Itoa(port)},
		filepath.Join(logDir, "serve.log")); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	// cloudflared with the connector token (0600 — the plist holds the token).
	if err := writeLaunchAgent(tunnelAgentPath(), tunnelAgentLabel,
		[]string{bin, "tunnel", "run", "--protocol", cloudflaredProtocol(), "--token", prov.Token},
		filepath.Join(logDir, "tunnel.log")); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	_ = os.WriteFile(tunnelURLPath(), []byte(prov.URL+"\n"), 0o600)

	launchctl("unload", serveAgentPath())
	launchctl("unload", tunnelAgentPath())
	if err := launchctl("load", serveAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: launchctl load serve: "+err.Error(), "gtmux tunnel: launchctl load serve: "+err.Error())
	}
	if err := launchctl("load", tunnelAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: launchctl load tunnel: "+err.Error(), "gtmux tunnel: launchctl load tunnel: "+err.Error())
	}

	printPairingBlock(prov.URL, token, name, port)
	i18n.Say(i18n.Dim+"Always-on enabled — reachable across reboots. Turn off: `gtmux tunnel --unservice`."+i18n.Reset,
		i18n.Dim+"Always-on 已开启，重启也可达。关闭：`gtmux tunnel --unservice`。"+i18n.Reset)
	return 0
}

// serveServiceInstall registers a LAN-only serve LaunchAgent (bind 0.0.0.0 so the
// phone reaches it on the same Wi-Fi) that survives reboots. This is the free
// "same Wi-Fi" remote mode; it removes any always-on tunnel agent first, since
// LAN and anywhere are mutually exclusive remote-access modes (the menu-bar app
// exposes them as one Off / Wi-Fi / Anywhere chooser).
func serveServiceInstall(port int) int {
	_ = resolveServeToken("") // ensure the persistent serve-token exists (0600)

	// drop the tunnel layer if present — this mode is LAN-only.
	launchctl("unload", tunnelAgentPath())
	_ = os.Remove(tunnelAgentPath())
	_ = os.Remove(tunnelURLPath())

	logDir := filepath.Join(homeDir(), ".local", "share", "gtmux")
	_ = os.MkdirAll(logDir, 0o755)
	if err := os.MkdirAll(launchAgentsDir(), 0o755); err != nil {
		i18n.Sae("gtmux serve: "+err.Error(), "gtmux serve: "+err.Error())
		return 1
	}
	if err := writeLaunchAgent(serveAgentPath(), serveAgentLabel,
		[]string{selfPath(), "serve", "--bind", "0.0.0.0", "--port", strconv.Itoa(port)},
		filepath.Join(logDir, "serve.log")); err != nil {
		i18n.Sae("gtmux serve: "+err.Error(), "gtmux serve: "+err.Error())
		return 1
	}
	launchctl("unload", serveAgentPath())
	if err := launchctl("load", serveAgentPath()); err != nil {
		i18n.Sae("gtmux serve: launchctl load: "+err.Error(), "gtmux serve: launchctl load: "+err.Error())
		return 1
	}
	i18n.Say("LAN access enabled — reachable on the same Wi-Fi across reboots. Turn off: `gtmux serve --unservice`.",
		"局域网访问已开启，同一 Wi-Fi 下重启也可达。关闭：`gtmux serve --unservice`。")
	return 0
}

// serviceRemoveAll unloads + removes whichever remote-access agents exist (the
// LAN serve and/or the always-on tunnel). It backs both `gtmux serve --unservice`
// and the menu-bar "Off" choice, so turning remote access off works from any mode.
func serviceRemoveAll() int {
	had := fileExists(serveAgentPath()) || fileExists(tunnelAgentPath())
	launchctl("unload", serveAgentPath())
	launchctl("unload", tunnelAgentPath())
	_ = os.Remove(serveAgentPath())
	_ = os.Remove(tunnelAgentPath())
	_ = os.Remove(tunnelURLPath())
	if had {
		i18n.Say("Remote access disabled — background services stopped and removed.",
			"远程访问已关闭，后台服务已停止并移除。")
	} else {
		i18n.Say("Remote access is not enabled.", "远程访问未开启。")
	}
	return 0
}

// tunnelServiceRemove unloads + deletes the always-on agents.
func tunnelServiceRemove() int {
	if !serviceInstalled() {
		i18n.Say("Always-on is not enabled.", "Always-on 未开启。")
		return 0
	}
	launchctl("unload", serveAgentPath())
	launchctl("unload", tunnelAgentPath())
	_ = os.Remove(serveAgentPath())
	_ = os.Remove(tunnelAgentPath())
	_ = os.Remove(tunnelURLPath())
	i18n.Say("Always-on disabled — the background tunnel + serve are stopped and removed.",
		"Always-on 已关闭，后台隧道与 serve 已停止并移除。")
	return 0
}

// tunnelServiceStatus reports whether always-on is active and at which URL.
func tunnelServiceStatus() int {
	if !serviceInstalled() {
		i18n.Say("Always-on: off  (run `gtmux tunnel --service` to enable, or `gtmux tunnel` for a foreground session)",
			"Always-on：关闭  （跑 `gtmux tunnel --service` 开启，或 `gtmux tunnel` 前台开一次）")
		return 0
	}
	loaded := launchctlLoaded(serveAgentLabel) && launchctlLoaded(tunnelAgentLabel)
	state := i18n.Tr("on", "开启")
	if !loaded {
		state = i18n.Tr("installed but not running (try re-login or --service again)",
			"已安装但未运行（重新登录或再跑 --service）")
	}
	i18n.Say("Always-on: "+state, "Always-on:"+state)
	if b, err := os.ReadFile(tunnelURLPath()); err == nil {
		fmt.Printf("  URL: %s", string(b))
	}
	return 0
}

// writeLaunchAgent writes a per-user LaunchAgent plist that runs args at login,
// keeping it alive, logging to logPath. Written 0600 (args may hold a token).
func writeLaunchAgent(path, label string, args []string, logPath string) error {
	var b []byte
	b = append(b, []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>`+xmlEsc(label)+`</string>
  <key>ProgramArguments</key><array>
`)...)
	for _, a := range args {
		b = append(b, []byte("    <string>"+xmlEsc(a)+"</string>\n")...)
	}
	b = append(b, []byte(`  </array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>`+xmlEsc(logPath)+`</string>
  <key>StandardErrorPath</key><string>`+xmlEsc(logPath)+`</string>
</dict></plist>
`)...)
	return os.WriteFile(path, b, 0o600)
}

func xmlEsc(s string) string {
	var buf bytes.Buffer
	_ = xml.EscapeText(&buf, []byte(s))
	return buf.String()
}

func launchctl(action, plist string) error {
	args := []string{action}
	if action == "load" || action == "unload" {
		args = append(args, "-w")
	}
	args = append(args, plist)
	return exec.Command("launchctl", args...).Run()
}

// launchctlLoaded reports whether a label is currently loaded.
func launchctlLoaded(label string) bool {
	return exec.Command("launchctl", "list", label).Run() == nil
}
