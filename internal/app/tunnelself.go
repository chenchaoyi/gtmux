package app

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"hash/crc32"
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
	selfTunnelAgentLabel = "com.gtmux.selftunnel"
	// Per-device reverse-tunnel port band on the shared VPS. Each Mac derives a
	// STABLE port in [selfPortBase, selfPortBase+selfPortSpan) from its device id, so
	// multiple Macs on the SAME gtmux Direct server never collide on one fixed port
	// (the old fixed 9000 was single-tenant). The VPS reverse proxy routes
	// <base>/p<port>/… → 127.0.0.1:<port>, giving each Mac its own pairing URL.
	// The band stays inside 20000–59999 so the proxy's port matcher can reject a
	// crafted path that would otherwise reach a system service.
	selfPortBase = 20000
	selfPortSpan = 40000
)

// selfTunnelPort is this Mac's stable VPS-side reverse-tunnel port, derived from the
// device id so it's the same across restarts but differs between Macs.
func selfTunnelPort() int {
	return selfPortBase + int(crc32.ChecksumIEEE([]byte(resolveDeviceID()))%selfPortSpan)
}

// selfTunnelPairURL turns the tunnel base ("https://tunnel.ccy.dev") into THIS
// device's pairing URL ("https://tunnel.ccy.dev/p<port>") — the path the VPS routes
// to this Mac's reverse port. The QR/phone use it as the base for every /api call
// (the app just string-concats "/api/…", so the prefix is preserved). The chisel
// DIAL target stays the bare base; only the pairing URL carries the /p<port> path.
func selfTunnelPairURL(base string) string {
	return strings.TrimRight(base, "/") + "/p" + strconv.Itoa(selfTunnelPort())
}

// connectedRe matches chisel's "Connected (Latency …)" line → the tunnel is up.
var connectedRe = regexp.MustCompile(`client: Connected`)

func selfTunnelAgentPath() string {
	return filepath.Join(launchAgentsDir(), selfTunnelAgentLabel+".plist")
}

// selfTunnelConfPath is the shared config the CLI reads and the menu-bar writes so
// both agree on the self-hosted server (URL + chisel secret). 0600 (holds the secret).
// Format: `url=…` / `secret=…` lines.
func selfTunnelConfPath() string {
	return filepath.Join(homeDir(), ".config", "gtmux", "selftunnel.conf")
}

// selfTunnelConfig reads the Direct server config from the env, else the shared
// config file (written by `gtmux tunnel --redeem <code>` or by a user who runs their
// OWN server), or explains how to get it and returns ok=false. The config is NEVER
// baked into the binary — Direct is gtmux's paid tunnel, so the server + its chisel
// secret are handed out ONLY on a valid access code (validated server-side), which
// lets the repo stay fully public.
func selfTunnelConfig() (url, secret string, ok bool) {
	url = strings.TrimSpace(os.Getenv("GTMUX_SELFTUNNEL_URL"))
	secret = strings.TrimSpace(os.Getenv("GTMUX_SELFTUNNEL_SECRET"))
	if url == "" || secret == "" {
		fu, fs := readSelfTunnelConf()
		if url == "" {
			url = fu
		}
		if secret == "" {
			secret = fs
		}
	}
	if url == "" || secret == "" {
		i18n.Sae("Direct isn't unlocked on this Mac. Redeem your access code:  gtmux tunnel --redeem <code>",
			"这台 Mac 还没解锁 Direct。用你的访问码解锁：  gtmux tunnel --redeem <码>")
		i18n.Sae("  (or point at your OWN server via GTMUX_SELFTUNNEL_URL + GTMUX_SELFTUNNEL_SECRET / "+selfTunnelConfPath()+" — see deploy/self-tunnel/README.md)",
			"  （或用 GTMUX_SELFTUNNEL_URL + GTMUX_SELFTUNNEL_SECRET / "+selfTunnelConfPath()+" 指向你自己的服务器 —— 见 deploy/self-tunnel/README.md）")
		return "", "", false
	}
	return url, secret, true
}

// writeSelfTunnelConf saves the Direct server config (0600 — it holds the secret) so
// selfTunnelConfig picks it up on the normal path. Written by `--redeem`.
func writeSelfTunnelConf(url, secret string) error {
	p := selfTunnelConfPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return os.WriteFile(p, []byte("url="+url+"\nsecret="+secret+"\n"), 0o600)
}

// redeemDirectCode validates a paid Direct access code with the control-plane Worker
// and, on success, writes the returned server config to selftunnel.conf — so Direct
// then works via the normal self-tunnel path. The server + its chisel secret are
// NEVER in the binary; the Worker hands them out only for a valid code.
func redeemDirectCode(code string) int {
	code = strings.TrimSpace(code)
	if code == "" {
		i18n.Sae("usage: gtmux tunnel --redeem <code>", "用法：gtmux tunnel --redeem <码>")
		return 2
	}
	url, secret, err := redeemDirect(code)
	if err != nil {
		if err == errInvalidCode {
			i18n.Sae("gtmux tunnel: that Direct code is invalid or has been revoked.",
				"gtmux tunnel: 这个 Direct 码无效或已被吊销。")
		} else {
			i18n.Sae("gtmux tunnel: couldn't reach the unlock service (network?): "+err.Error(),
				"gtmux tunnel: 连不上解锁服务（网络？）："+err.Error())
		}
		return 1
	}
	if err := writeSelfTunnelConf(url, secret); err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	i18n.Say("✓ Direct unlocked on this Mac. Turn it on: the menu bar's Anywhere → Direct, or `gtmux tunnel --backend self`.",
		"✓ 这台 Mac 已解锁 Direct。开启：菜单栏 Anywhere → Direct，或 `gtmux tunnel --backend self`。")
	return 0
}

// errInvalidCode marks a 403 from the unlock service (bad/revoked code) so the caller
// can say so precisely instead of blaming the network.
var errInvalidCode = fmt.Errorf("invalid or revoked code")

// redeemDirect POSTs the code to the control-plane Worker's /direct/redeem and returns
// the Direct server config on success. Retries transient network/5xx across the
// primary + fallback bases (same resilience as provisionTunnel).
func redeemDirect(code string) (url, secret string, err error) {
	api := tunnelAPI()
	bases := []string{api}
	if fb := tunnelAPIFallback(); fb != "" && fb != api {
		bases = append(bases, fb)
	}
	body, _ := json.Marshal(map[string]string{"code": code, "deviceId": resolveDeviceID()})
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		for _, base := range bases {
			req, e := http.NewRequest("POST", strings.TrimRight(base, "/")+"/direct/redeem", bytes.NewReader(body))
			if e != nil {
				lastErr = e
				continue
			}
			req.Header.Set("Content-Type", "application/json")
			res, e := (&http.Client{Timeout: 20 * time.Second}).Do(req)
			if e != nil {
				lastErr = e
				continue // network → retry
			}
			data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
			_ = res.Body.Close()
			if res.StatusCode == 403 {
				return "", "", errInvalidCode
			}
			if res.StatusCode != 200 {
				lastErr = fmt.Errorf("HTTP %d", res.StatusCode)
				if res.StatusCode < 500 {
					return "", "", lastErr // 4xx won't improve
				}
				continue
			}
			var r struct {
				URL    string `json:"url"`
				Secret string `json:"secret"`
			}
			if e := json.Unmarshal(data, &r); e != nil {
				lastErr = e
				continue
			}
			if r.URL == "" || r.Secret == "" {
				return "", "", fmt.Errorf("incomplete redeem response")
			}
			return r.URL, r.Secret, nil
		}
		time.Sleep(time.Duration(attempt+1) * 500 * time.Millisecond)
	}
	return "", "", lastErr
}

// readSelfTunnelConf parses url= / secret= from the shared config file ("" when absent).
func readSelfTunnelConf() (url, secret string) {
	b, err := os.ReadFile(selfTunnelConfPath())
	if err != nil {
		return "", ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if k, v, found := strings.Cut(line, "="); found {
			switch strings.TrimSpace(k) {
			case "url":
				url = strings.TrimSpace(v)
			case "secret":
				secret = strings.TrimSpace(v)
			}
		}
	}
	return url, secret
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
	pairURL := selfTunnelPairURL(url) // per-device path so multiple Macs don't collide
	_ = os.WriteFile(tunnelURLPath(), []byte(pairURL+"\n"), 0o600)
	if !serviceInstalled() {
		defer func() { _ = os.Remove(tunnelURLPath()) }()
	}
	i18n.Say("Starting your self-hosted tunnel…", "正在启动自建隧道…")
	// Dial the bare base; reverse-forward this device's own VPS port → local serve.
	args := []string{"client", "--keepalive", "25s", url,
		fmt.Sprintf("R:127.0.0.1:%d:localhost:%d", selfTunnelPort(), port)}
	return runChiselClient(bin, args, secret, func() {
		printTunnelPairing(pairURL, token, name, port, true)
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
	_ = os.WriteFile(tunnelURLPath(), []byte(selfTunnelPairURL(url)+"\n"), 0o600)

	// Backends are mutually exclusive — retire a Cloudflare tunnel agent if present
	// so switching self↔cloudflare never leaves two tunnels fighting for the serve.
	if fileExists(tunnelAgentPath()) {
		launchctl("unload", tunnelAgentPath())
		_ = os.Remove(tunnelAgentPath())
	}
	launchctl("unload", serveAgentPath())
	launchctl("unload", selfTunnelAgentPath())
	if err := launchctl("load", serveAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: launchctl load serve: "+err.Error(), "gtmux tunnel: launchctl load serve: "+err.Error())
	}
	if err := launchctl("load", selfTunnelAgentPath()); err != nil {
		i18n.Sae("gtmux tunnel: launchctl load selftunnel: "+err.Error(), "gtmux tunnel: launchctl load selftunnel: "+err.Error())
	}
	printPairingBlock(selfTunnelPairURL(url), resolveServeToken(""), name, port)
	i18n.Say(i18n.Dim+"Always-on (self-hosted) enabled. Turn off: `gtmux tunnel --unservice`."+i18n.Reset,
		i18n.Dim+"Always-on（自建）已开启。关闭：`gtmux tunnel --unservice`。"+i18n.Reset)
	return 0
}

// writeChiselAgent writes a launchd plist for the chisel client, carrying the auth
// secret in EnvironmentVariables (so it isn't visible in `ps`). 0600.
func writeChiselAgent(path, bin, url, secret string, port int, logPath string) error {
	remote := fmt.Sprintf("R:127.0.0.1:%d:localhost:%d", selfTunnelPort(), port)
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
