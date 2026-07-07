package app

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Self-hosted tunnel backend (`gtmux tunnel --backend self`). Instead of Cloudflare,
// the Mac dials out over 443/WebSocket (chisel) to the user's OWN VPS + domain —
// indistinguishable from ordinary HTTPS, so it survives networks that DNS-hijack
// Cloudflare's edge (`*.argotunnel.com`). The user runs the server side (chisel +
// a TLS reverse proxy) on their VPS; see deploy/self-tunnel/. Config is manual (own
// server): GTMUX_SELFTUNNEL_URL (https://tunnel.example.com) + GTMUX_SELFTUNNEL_SECRET
// (chisel auth, user:pass).

const (
	chiselVersion        = "1.10.1"
	selfRemotePort       = 9000 // the VPS-side loopback port the reverse tunnel exposes → the reverse proxy fronts it
	selfTunnelAgentLabel = "com.gtmux.selftunnel"
)

// connectedRe matches chisel's "Connected (Latency …)" line → the tunnel is up.
var connectedRe = regexp.MustCompile(`client: Connected`)

func selfTunnelAgentPath() string {
	return filepath.Join(launchAgentsDir(), selfTunnelAgentLabel+".plist")
}

// selfTunnelConfig reads the user's self-hosted server config, or explains what to
// set and returns ok=false.
func selfTunnelConfig() (url, secret string, ok bool) {
	url = strings.TrimSpace(os.Getenv("GTMUX_SELFTUNNEL_URL"))
	secret = strings.TrimSpace(os.Getenv("GTMUX_SELFTUNNEL_SECRET"))
	if url == "" || secret == "" {
		i18n.Sae("gtmux tunnel --backend self needs YOUR server: set GTMUX_SELFTUNNEL_URL"+
			" (e.g. https://tunnel.example.com) and GTMUX_SELFTUNNEL_SECRET (chisel auth user:pass). See deploy/self-tunnel/README.md.",
			"gtmux tunnel --backend self 需要你自己的服务器：设置 GTMUX_SELFTUNNEL_URL"+
				"（如 https://tunnel.example.com）和 GTMUX_SELFTUNNEL_SECRET（chisel 认证 user:pass）。见 deploy/self-tunnel/README.md。")
		return "", "", false
	}
	return url, secret, true
}

// tunnelSelf runs the self-hosted tunnel in the foreground: it ensures chisel, starts
// the read-only radar if needed, dials the user's VPS, and prints the pairing block
// with the user's own domain (the phone pairs to {url, token} exactly as with Cloudflare).
func tunnelSelf(port int, name string) int {
	url, secret, ok := selfTunnelConfig()
	if !ok {
		return 2
	}
	bin := ensureChisel()
	if bin == "" {
		return 1
	}
	token := startLocalRadar(port)
	_ = os.WriteFile(tunnelURLPath(), []byte(url+"\n"), 0o600)
	if !serviceInstalled() {
		defer func() { _ = os.Remove(tunnelURLPath()) }()
	}
	i18n.Say("Starting your self-hosted tunnel…", "正在启动自建隧道…")
	args := []string{"client", "--keepalive", "25s", url,
		fmt.Sprintf("R:127.0.0.1:%d:localhost:%d", selfRemotePort, port)}
	return runChiselClient(bin, args, secret, func() {
		printTunnelPairing(url, token, name, port, true)
	})
}

// tunnelSelfServiceInstall registers the always-on self-hosted tunnel: a serve
// LaunchAgent (loopback radar) + a chisel LaunchAgent dialing the user's VPS.
func tunnelSelfServiceInstall(port int, name string, yes bool) int {
	url, secret, ok := selfTunnelConfig()
	if !ok {
		return 2
	}
	bin := ensureChisel()
	if bin == "" {
		return 1
	}
	if !yes {
		i18n.Say(i18n.Bold+"Keep the self-hosted tunnel ON across reboots?"+i18n.Reset,
			i18n.Bold+"让自建隧道重启后也保持开启？"+i18n.Reset)
		i18n.Say(i18n.Dim+"  Registers two background services (chisel + gtmux serve). It's a standing"+i18n.Reset,
			i18n.Dim+"  会注册两个后台服务（chisel + gtmux serve）。这是一个持续暴露"+i18n.Reset)
		i18n.Say(i18n.Dim+"  exposure (token-gated). Turn off with `gtmux tunnel --unservice`."+i18n.Reset,
			i18n.Dim+"（有 token 把关）。用 `gtmux tunnel --unservice` 关闭。"+i18n.Reset)
		if !confirm(i18n.Tr("Enable always-on? [y/N] ", "开启 always-on？[y/N] ")) {
			i18n.Say("Skipped.", "已跳过。")
			return 0
		}
	}
	logDir := filepath.Join(homeDir(), ".local", "share", "gtmux")
	_ = os.MkdirAll(logDir, 0o755)
	// serve on loopback (the tunnel reaches it locally).
	if err := writeLaunchAgent(serveAgentPath(), serveAgentLabel,
		[]string{selfPath(), "serve", "--bind", "127.0.0.1", "--port", strconv.Itoa(port)},
		filepath.Join(logDir, "serve.log")); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	// chisel client with AUTH in the plist's EnvironmentVariables (0600 plist), NOT argv.
	if err := writeChiselAgent(selfTunnelAgentPath(), bin, url, secret, port,
		filepath.Join(logDir, "selftunnel.log")); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	_ = os.WriteFile(tunnelURLPath(), []byte(url+"\n"), 0o600)

	launchctl("unload", serveAgentPath())
	launchctl("unload", selfTunnelAgentPath())
	if err := launchctl("load", serveAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: launchctl load serve: "+err.Error(), "gtmux tunnel: launchctl load serve: "+err.Error())
	}
	if err := launchctl("load", selfTunnelAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: launchctl load selftunnel: "+err.Error(), "gtmux tunnel: launchctl load selftunnel: "+err.Error())
	}
	printPairingBlock(url, resolveServeToken(""), name, port)
	i18n.Say(i18n.Dim+"Always-on (self-hosted) enabled. Turn off: `gtmux tunnel --unservice`."+i18n.Reset,
		i18n.Dim+"Always-on（自建）已开启。关闭：`gtmux tunnel --unservice`。"+i18n.Reset)
	return 0
}

// writeChiselAgent writes a launchd plist for the chisel client, carrying the auth
// secret in EnvironmentVariables (so it isn't visible in `ps`). 0600.
func writeChiselAgent(path, bin, url, secret string, port int, logPath string) error {
	remote := fmt.Sprintf("R:127.0.0.1:%d:localhost:%d", selfRemotePort, port)
	plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>` + xmlEsc(selfTunnelAgentLabel) + `</string>
  <key>ProgramArguments</key><array>
    <string>` + xmlEsc(bin) + `</string>
    <string>client</string><string>--keepalive</string><string>25s</string>
    <string>` + xmlEsc(url) + `</string>
    <string>` + xmlEsc(remote) + `</string>
  </array>
  <key>EnvironmentVariables</key><dict><key>AUTH</key><string>` + xmlEsc(secret) + `</string></dict>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>StandardOutPath</key><string>` + xmlEsc(logPath) + `</string>
  <key>StandardErrorPath</key><string>` + xmlEsc(logPath) + `</string>
</dict></plist>
`
	return os.WriteFile(path, []byte(plist), 0o600)
}

// runChiselClient runs chisel with AUTH in the env (not argv), echoes problems, and
// calls onReady on the first "Connected" line. Mirrors runCloudflared but adds the
// auth env and its own ready detection.
func runChiselClient(bin string, args []string, secret string, onReady func()) int {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), "AUTH="+secret)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	if err := cmd.Start(); err != nil {
		i18n.Sae("gtmux tunnel: failed to start the tunnel client: "+err.Error(),
			"gtmux tunnel: 启动隧道客户端失败："+err.Error())
		return 1
	}
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		if _, ok := <-sigc; ok && cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	}()
	ready := false
	verbose := os.Getenv("GTMUX_TUNNEL_DEBUG") != ""
	sc := bufio.NewScanner(stderr)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if verbose || cloudflaredProblem(line) {
			fmt.Fprintln(os.Stderr, i18n.Dim+line+i18n.Reset)
		}
		if !ready && connectedRe.MatchString(line) {
			onReady()
			ready = true
		}
	}
	err = cmd.Wait()
	signal.Stop(sigc)
	close(sigc)
	if err != nil && !ready {
		i18n.Sae("gtmux tunnel: the tunnel client exited: "+err.Error(),
			"gtmux tunnel: 隧道客户端退出："+err.Error())
		return 1
	}
	return 0
}

// chiselPath returns a usable jpillora/chisel binary: the gtmux-managed copy, or a
// `chisel` on PATH that is the TUNNEL (not Homebrew's Facebook LLDB "chisel"). "" if none.
func chiselPath() string {
	managed := filepath.Join(localBinDir(), "gtmux-chisel")
	if fileExists(managed) {
		return managed
	}
	if p, err := exec.LookPath("chisel"); err == nil {
		// Distinguish jpillora chisel (understands `--version` → a version string)
		// from Homebrew's LLDB "chisel" (a Python tool that doesn't).
		if out, err := exec.Command(p, "--version").Output(); err == nil && len(out) > 0 && out[0] >= '0' && out[0] <= '9' {
			return p
		}
	}
	return ""
}

// ensureChisel returns a chisel binary, downloading the correct jpillora release to
// ~/.local/bin/gtmux-chisel when none is present. "" if it couldn't be set up.
func ensureChisel() string {
	if p := chiselPath(); p != "" {
		return p
	}
	i18n.Say("Fetching the tunnel client (chisel)…", "正在获取隧道客户端（chisel）…")
	dst := filepath.Join(localBinDir(), "gtmux-chisel")
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return ""
	}
	url := fmt.Sprintf("https://github.com/jpillora/chisel/releases/download/v%s/chisel_%s_%s_%s.gz",
		chiselVersion, chiselVersion, runtime.GOOS, runtime.GOARCH)
	if err := downloadGzBinary(url, dst); err != nil {
		i18n.Sae("gtmux tunnel: couldn't fetch chisel: "+err.Error()+" — install it from https://github.com/jpillora/chisel/releases",
			"gtmux tunnel: 获取 chisel 失败："+err.Error()+" —— 从 https://github.com/jpillora/chisel/releases 手动安装")
		return ""
	}
	i18n.Say(i18n.Dim+"Installed chisel → "+dst+i18n.Reset, i18n.Dim+"已安装 chisel → "+dst+i18n.Reset)
	return dst
}

// downloadGzBinary fetches a gzip-compressed binary and writes it executable to dst.
func downloadGzBinary(url, dst string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer gz.Close()
	tmp := dst + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, gz); err != nil { //nolint:gosec // release artifact, size-bounded by the client timeout
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}

func localBinDir() string { return filepath.Join(homeDir(), ".local", "bin") }
