package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// cmdTunnel implements `gtmux tunnel` — expose the read-only radar to the phone
// from ANYWHERE (no LAN, no VPN app) by running an outbound reverse tunnel on the
// Mac. The tunnel client (cloudflared) lives only here; the phone app just gets a
// `{url, token}` pairing, so the transport never touches the app.
//
// Default = HOSTED: the gtmux control-plane Worker provisions a STABLE
// `gtmux-<id>.ccy.dev` named tunnel for this Mac, so the phone pairs ONCE and
// keeps reaching the Mac across restarts (the address never changes). `--quick`
// uses an account-less Cloudflare quick tunnel whose URL rotates each run.
func cmdTunnel(args []string) int {
	port := defaultServePort
	name, _ := os.Hostname()
	if name == "" {
		name = "Mac"
	}
	quick := false
	yes := false
	// backend: "cloudflare" (default, zero-config hosted) or "self" (a WebSocket-
	// over-443 tunnel to YOUR OWN VPS + domain — survives networks that block
	// Cloudflare's edge). Env default, --backend overrides.
	backend := strings.TrimSpace(os.Getenv("GTMUX_TUNNEL_BACKEND"))
	var service string // "install" | "remove" | "status"
	var redeem string  // Direct access code to unlock (writes selftunnel.conf)

	for i := 0; i < len(args); i++ {
		a := args[i]
		next := func() (string, bool) {
			if i+1 < len(args) {
				i++
				return args[i], true
			}
			return "", false
		}
		switch {
		case a == "-h" || a == "--help":
			tunnelUsage()
			return 0
		case a == "--redeem":
			v, ok := next()
			if !ok {
				return tunnelUsageErr()
			}
			redeem = v
		case strings.HasPrefix(a, "--redeem="):
			redeem = strings.TrimPrefix(a, "--redeem=")
		case a == "--backend":
			v, ok := next()
			if !ok {
				return tunnelUsageErr()
			}
			backend = v
		case strings.HasPrefix(a, "--backend="):
			backend = strings.TrimPrefix(a, "--backend=")
		case a == "--quick":
			quick = true
		case a == "--service":
			service = "install"
		case a == "--unservice":
			service = "remove"
		case a == "--status":
			service = "status"
		case a == "-y" || a == "--yes":
			yes = true
		case a == "--port" || a == "-p":
			v, ok := next()
			if !ok {
				return tunnelUsageErr()
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 || n > 65535 {
				i18n.Sae("gtmux tunnel: invalid --port", "gtmux tunnel: 无效的 --port")
				return 2
			}
			port = n
		case strings.HasPrefix(a, "--port="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--port="))
			if err != nil || n <= 0 || n > 65535 {
				i18n.Sae("gtmux tunnel: invalid --port", "gtmux tunnel: 无效的 --port")
				return 2
			}
			port = n
		case a == "--name":
			v, ok := next()
			if !ok {
				return tunnelUsageErr()
			}
			name = v
		case strings.HasPrefix(a, "--name="):
			name = strings.TrimPrefix(a, "--name=")
		default:
			i18n.Sae("gtmux tunnel: unknown option '"+a+"'", "gtmux tunnel: 未知选项 '"+a+"'")
			return 2
		}
	}

	// Unlock Direct: validate the access code server-side, write the config it hands
	// back, then exit (the user enables Direct from the menu bar or --backend self).
	if redeem != "" {
		return redeemDirectCode(redeem)
	}

	if backend != "" && backend != "cloudflare" && backend != "self" {
		i18n.Sae("gtmux tunnel: --backend must be 'cloudflare' or 'self'", "gtmux tunnel: --backend 只能是 'cloudflare' 或 'self'")
		return 2
	}
	switch service {
	case "install":
		if backend == "self" {
			return tunnelSelfServiceInstall(port, name, yes)
		}
		return tunnelServiceInstall(port, name, yes)
	case "remove":
		return tunnelServiceRemove()
	case "status":
		return tunnelServiceStatus()
	}
	if backend == "self" {
		return tunnelSelf(port, name)
	}
	// Dual-tunnel guard: if the always-on tunnel (the menu-bar's "Anywhere" mode)
	// is already running, a foreground `gtmux tunnel` would start a SECOND,
	// redundant tunnel. Print the existing address instead so you can just pair.
	if alreadyServingTunnel() {
		return reuseRunningTunnel(name, port)
	}
	if quick {
		return tunnelQuick(port, name, yes)
	}
	return tunnelHosted(port, name, yes)
}

// alreadyServingTunnel reports whether the always-on tunnel LaunchAgent is both
// installed AND loaded — i.e. the Mac is already reachable from anywhere, so a
// foreground tunnel is redundant.
func alreadyServingTunnel() bool {
	return serviceInstalled() && launchctlLoaded(tunnelAgentLabel)
}

// reuseRunningTunnel tells the user the always-on tunnel is already up and prints
// its existing pairing block (URL + QR) instead of starting a second tunnel.
func reuseRunningTunnel(name string, port int) int {
	url := ""
	if b, err := os.ReadFile(tunnelURLPath()); err == nil {
		url = strings.TrimSpace(string(b))
	}
	i18n.Say("Always-on tunnel is already running (enabled from the menu bar) — reusing it instead of starting another.",
		"always-on 隧道已在运行（从菜单栏开启）—— 直接复用，不再启动第二条。")
	if url == "" {
		// Running but URL file missing (rare) — point at --status rather than guess.
		i18n.Say("  Run `gtmux tunnel --status` for its address, or `gtmux tunnel --unservice` to turn it off.",
			"  跑 `gtmux tunnel --status` 看地址，或 `gtmux tunnel --unservice` 关闭。")
		return 0
	}
	printPairingBlock(url, resolveServeToken(""), name, port)
	i18n.Say(i18n.Dim+"This is the always-on tunnel. Turn it off with `gtmux tunnel --unservice`."+i18n.Reset,
		i18n.Dim+"这是 always-on 隧道。用 `gtmux tunnel --unservice` 关闭。"+i18n.Reset)
	return 0
}

// --- hosted (stable URL via the control-plane Worker) ---------------------------

var registeredRe = regexp.MustCompile(`Registered tunnel connection`)

func tunnelHosted(port int, name string, yes bool) int {
	reg := tunnelRegSecret()
	if reg == "" {
		i18n.Sae("gtmux tunnel: hosted mode isn't configured in this build. Use `gtmux tunnel --quick` for an ephemeral tunnel, or set GTMUX_TUNNEL_REG.",
			"gtmux tunnel: 此构建未启用托管模式。用 `gtmux tunnel --quick` 走临时隧道，或设置 GTMUX_TUNNEL_REG。")
		return 2
	}
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		if bin = ensureCloudflared(yes); bin == "" {
			return 1
		}
	}

	i18n.Say("Requesting your stable tunnel address…", "正在申请你的固定隧道地址…")
	prov, err := provisionTunnel(tunnelAPI(), reg, resolveDeviceID(), name)
	if err != nil {
		en, zh := friendlyTunnelError(err)
		i18n.Sae(en, zh)
		tunnelDebugf("provision failed: %v", err) // raw detail only under GTMUX_TUNNEL_DEBUG
		return 1
	}

	token := startLocalRadar(port)
	// Record the live tunnel URL so the menu-bar "Pair phone" QR can hand the
	// phone the address that actually works (serve here is loopback-only — a LAN
	// IP wouldn't be reachable). Clean up on exit unless the always-on service
	// owns the file.
	_ = os.WriteFile(tunnelURLPath(), []byte(prov.URL+"\n"), 0o600)
	if !serviceInstalled() {
		defer func() { _ = os.Remove(tunnelURLPath()) }()
	}
	i18n.Say("Starting your tunnel…", "正在启动隧道…")
	return runCloudflared(bin, []string{"tunnel", "run", "--protocol", cloudflaredProtocol(), "--token", prov.Token}, registeredRe, func(string) {
		printTunnelPairing(prov.URL, token, name, port, true)
	})
}

// friendlyTunnelError turns a raw provision/network error into a calm, user-facing
// message — no internal service URLs, Go error strings, or "provision" jargon. The
// raw detail is still available via GTMUX_TUNNEL_DEBUG for diagnosis.
func friendlyTunnelError(err error) (en, zh string) {
	s := strings.ToLower(err.Error())
	contains := func(subs ...string) bool {
		for _, sub := range subs {
			if strings.Contains(s, sub) {
				return true
			}
		}
		return false
	}
	switch {
	case contains("eof", "timeout", "deadline exceeded", "connection reset",
		"connection refused", "no such host", "network is unreachable", "dial tcp",
		"tls", "i/o timeout"):
		return "Couldn't reach the tunnel service — check your internet and try again (or `gtmux tunnel --quick` for a one-off link).",
			"连不上隧道服务 —— 请检查网络后重试（或用 `gtmux tunnel --quick` 走一次性链接）。"
	case contains("http 4"):
		return "The tunnel service turned down this request. Try `gtmux tunnel --quick` for a one-off link.",
			"隧道服务拒绝了此请求。可用 `gtmux tunnel --quick` 走一次性链接。"
	default:
		return "Couldn't set up the tunnel just now — please try again in a moment.",
			"暂时无法建立隧道，请稍后重试。"
	}
}

// tunnelDebugf prints internal detail (raw errors, service URLs) to stderr ONLY when
// GTMUX_TUNNEL_DEBUG is set — so normal output stays clean but issues stay diagnosable.
func tunnelDebugf(format string, a ...any) {
	if os.Getenv("GTMUX_TUNNEL_DEBUG") == "" {
		return
	}
	fmt.Fprintf(os.Stderr, i18n.Dim+"[tunnel] "+format+i18n.Reset+"\n", a...)
}

// provisionResp is the control-plane Worker's /provision reply.
type provisionResp struct {
	URL      string `json:"url"`
	Hostname string `json:"hostname"`
	Token    string `json:"token"`
}

// provisionTunnel calls /provision, retrying across the primary + fallback base
// and a few attempts — flaky networks reset the connection (EOF) intermittently,
// and a single shot shouldn't fail the whole command. A 4xx (e.g. bad reg gate)
// fails fast; network errors + 5xx retry.
func provisionTunnel(api, reg, deviceID, name string) (*provisionResp, error) {
	bases := []string{api}
	if fb := tunnelAPIFallback(); fb != "" && fb != api {
		bases = append(bases, fb)
	}
	var lastErr error
	for attempt := 0; attempt < 4; attempt++ {
		for _, base := range bases {
			p, retryable, err := provisionOnce(base, reg, deviceID, name)
			if err == nil {
				return p, nil
			}
			lastErr = err
			if !retryable {
				return nil, err
			}
		}
		time.Sleep(time.Duration(attempt+1) * 700 * time.Millisecond)
	}
	return nil, lastErr
}

// provisionOnce makes one /provision call. retryable is true for network errors
// and 5xx (transient), false for 4xx / malformed responses (won't improve).
func provisionOnce(base, reg, deviceID, name string) (p *provisionResp, retryable bool, err error) {
	body, _ := json.Marshal(map[string]string{"deviceId": deviceID, "name": name})
	req, err := http.NewRequest("POST", strings.TrimRight(base, "/")+"/provision", bytes.NewReader(body))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-gtmux-reg", reg)
	res, err := (&http.Client{Timeout: 20 * time.Second}).Do(req)
	if err != nil {
		return nil, true, err // network/reset → retry
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<16))
	if res.StatusCode != 200 {
		return nil, res.StatusCode >= 500,
			fmt.Errorf("HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(data)))
	}
	var resp provisionResp
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, true, err
	}
	if resp.Token == "" || resp.URL == "" {
		return nil, false, fmt.Errorf("incomplete provision response")
	}
	return &resp, false, nil
}

// resolveDeviceID returns a stable random id for this Mac (so re-provisioning
// reuses the same tunnel/hostname), generating + persisting it on first run.
func resolveDeviceID() string {
	path := filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "tunnel-device-id")
	if b, err := os.ReadFile(path); err == nil {
		if id := strings.TrimSpace(string(b)); len(id) >= 16 {
			return id
		}
	}
	id := randToken() + randToken() // 64 hex chars
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, []byte(id+"\n"), 0o600)
	}
	return id
}

// --- quick (account-less, ephemeral URL) ---------------------------------------

// tryCloudflareRe matches the public URL cloudflared prints for a quick tunnel.
var tryCloudflareRe = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

func tunnelQuick(port int, name string, yes bool) int {
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		if bin = ensureCloudflared(yes); bin == "" {
			return 1
		}
	}
	token := startLocalRadar(port)
	i18n.Say("Opening a quick tunnel (no account, ephemeral URL)…",
		"正在打开临时隧道（免账号、临时地址）…")
	args := []string{"tunnel", "--no-autoupdate", "--protocol", cloudflaredProtocol(), "--url", fmt.Sprintf("http://localhost:%d", port)}
	return runCloudflared(bin, args, tryCloudflareRe, func(line string) {
		printTunnelPairing(tryCloudflareRe.FindString(line), token, name, port, false)
	})
}

// --- shared cloudflared runner -------------------------------------------------

// cloudflaredProtocol picks the transport cloudflared uses to reach Cloudflare's
// edge. Default "http2" (TCP/443) instead of cloudflared's own QUIC default:
// QUIC rides UDP/7844, which corporate/campus networks routinely block, and when
// it's blocked the tunnel dies silently ("failed to dial to edge with quic:
// timeout") — the phone then can't reach the Mac at all. http2 works wherever
// HTTPS does, and gtmux's SSE/JSON traffic is far too light for QUIC's latency
// edge to matter. Override with GTMUX_TUNNEL_PROTOCOL (e.g. "quic" or "auto").
func cloudflaredProtocol() string {
	if p := strings.TrimSpace(os.Getenv("GTMUX_TUNNEL_PROTOCOL")); p != "" {
		return p
	}
	return "http2"
}

// startLocalRadar makes sure something read-only answers on the port: if `gtmux
// serve` is already up we tunnel to it (token matches — same file); otherwise we
// start the radar in-process bound to loopback. Returns the serve token.
func startLocalRadar(port int) string {
	token := resolveServeToken("")
	if portInUse(port) {
		i18n.Say(i18n.Dim+"Using the radar already running here."+i18n.Reset,
			i18n.Dim+"复用本机已在运行的雷达。"+i18n.Reset)
		return token
	}
	srv := newServeServer("127.0.0.1", port, token, "", "")
	go func() { _ = srv.ListenAndServe() }()
	for i := 0; i < 100 && !portInUse(port); i++ {
		time.Sleep(20 * time.Millisecond)
	}
	i18n.Say(i18n.Dim+"Local radar ready."+i18n.Reset, i18n.Dim+"本机雷达已就绪。"+i18n.Reset)
	return token
}

// runCloudflared runs cloudflared, echoes its log dimmed, calls onReady once on
// the first line matching readyRe, and relays Ctrl-C for a clean shutdown.
func runCloudflared(bin string, args []string, readyRe *regexp.Regexp, onReady func(line string)) int {
	cmd := exec.Command(bin, args...)
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
		// cloudflared's INF chatter (tunnel id, version, GOOS, ICMP proxy, the
		// connectivity-pre-checks box, DNS tables, …) is internal noise to an ordinary
		// user — suppress it both before AND after the QR, surfacing only real
		// warnings/errors. GTMUX_TUNNEL_DEBUG shows everything for diagnosis.
		if verbose || cloudflaredProblem(line) {
			fmt.Fprintln(os.Stderr, i18n.Dim+line+i18n.Reset)
		}
		if !ready && readyRe.MatchString(line) {
			onReady(line)
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

// cloudflaredProblem reports whether a cloudflared log line is a warning/error
// worth surfacing after the QR (its levels are INF / WRN / ERR).
func cloudflaredProblem(line string) bool {
	return strings.Contains(line, " ERR ") || strings.Contains(line, " WRN ") ||
		strings.Contains(strings.ToLower(line), "error")
}

// ensureCloudflared installs the Cloudflare tunnel client when it's missing.
// Returns the resolved path, or "" if it couldn't be set up. `yes` means
// non-interactive consent to install (the menu-bar "Anywhere" toggle runs
// `tunnel --service --yes` with NO TTY — without this, `confirm` would always
// decline and Anywhere could never come up on a fresh Mac). Failure reasons go to
// STDERR so the app can surface them (it maps the CLI's stderr to its error banner).
func ensureCloudflared(yes bool) string {
	i18n.Say("cloudflared isn't installed — it's the Cloudflare tunnel client (one binary, Mac-side only; the mobile app never touches it).",
		"未检测到 cloudflared，它是 Cloudflare 隧道客户端（一个二进制，只在 Mac 上跑；手机 App 完全不碰它）。")
	if _, err := exec.LookPath("brew"); err != nil {
		i18n.Sae("Anywhere needs cloudflared, and Homebrew isn't installed to fetch it. Install cloudflared, then retry: https://github.com/cloudflare/cloudflared/releases",
			"任意网络访问需要 cloudflared，但未安装 Homebrew 来获取它。请手动安装 cloudflared 后重试：https://github.com/cloudflare/cloudflared/releases")
		return ""
	}
	if !yes && !confirm(i18n.Tr("Install it now with `brew install cloudflared`? [Y/n] ",
		"现在用 `brew install cloudflared` 安装？[Y/n] ")) {
		i18n.Say("Skipped. Install it yourself, then re-run `gtmux tunnel`.",
			"已跳过。自行安装后重试 `gtmux tunnel`。")
		return ""
	}
	i18n.Say("Installing cloudflared (brew install cloudflared)…", "正在安装 cloudflared（brew install cloudflared）…")
	c := exec.Command("brew", "install", "cloudflared")
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := c.Run(); err != nil {
		i18n.Sae("gtmux tunnel: `brew install cloudflared` failed: "+err.Error(),
			"gtmux tunnel: `brew install cloudflared` 失败："+err.Error())
		return ""
	}
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		i18n.Sae("gtmux tunnel: cloudflared still not found after install.",
			"gtmux tunnel: 安装后仍未找到 cloudflared。")
		return ""
	}
	return bin
}

// portInUse reports whether something is already listening on 127.0.0.1:port.
func portInUse(port int) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

// printPairingBlock prints the URL and a scannable pairing QR. It prefers a
// SHORT-LIVED enroll code (v2 `{v,url,enrollCode,name}`) minted from the local
// radar, so the QR isn't a lasting credential; if minting fails (an old serve, or
// it isn't up yet) it falls back to the legacy v1 `{v,url,token,name}` so pairing
// still works. The menu-bar app encodes the same shape.
func printPairingBlock(url, token, name string, port int) {
	code := mintEnrollCode(port, token)
	payload := pairingPayload(url, token, code, name)

	fmt.Println()
	i18n.Say(i18n.Bold+"Your Mac is reachable from anywhere now:"+i18n.Reset,
		i18n.Bold+"你的 Mac 现在可从任何地方访问："+i18n.Reset)
	fmt.Printf("  URL:   %s\n", url)
	if code != "" {
		i18n.Say("  pairing code: a one-time code (expires in 5 min) — scan to pair, not the token",
			"  配对码：一次性，5 分钟内有效 —— 扫码配对，不再暴露长期 token")
	} else {
		fmt.Printf("  token: %s\n", token)
	}
	// Browser mirror: the same tunnel serves the view-only web UI, reachable from
	// any network. Reuse the one-time code for a browser pairing link.
	i18n.Say("  or open in a browser (view-only mirror):",
		"  或在浏览器里打开（只读镜像）：")
	if code != "" {
		fmt.Printf("    %s/#c=%s\n", url, code)
	} else {
		fmt.Printf("    %s/\n", url)
	}
	fmt.Println()
	i18n.Say("Scan this in the gtmux mobile app (Pair → Scan):",
		"在 gtmux 手机 App 里扫码（配对 → 扫一扫）：")
	printBrandQR(os.Stdout, string(payload))
}

// pairingPayload builds the QR JSON: the secure v2 `{enrollCode}` shape when a
// code was minted, else the legacy v1 `{token}` shape so pairing still works on a
// radar too old to mint codes.
//
// v2 OMITS `name` on purpose: it's only the server display label, and the phone
// derives a good label from the URL host when it's absent (PairingScreen). Every
// dropped field shrinks the QR module count — the only safe way to make the
// SQUARE terminal QR smaller (see the footgun note in qr.go).
func pairingPayload(url, token, code, name string) []byte {
	if code != "" {
		b, _ := json.Marshal(struct {
			V          int    `json:"v"`
			URL        string `json:"url"`
			EnrollCode string `json:"enrollCode"`
		}{2, url, code})
		return b
	}
	b, _ := json.Marshal(struct {
		V     int    `json:"v"`
		URL   string `json:"url"`
		Token string `json:"token"`
		Name  string `json:"name"`
	}{1, url, token, name})
	return b
}

// mintEnrollCode asks the local radar for a short-lived single-use pairing code
// (POST /api/enroll/mint, authenticated with the master token). Returns "" if the
// radar is unreachable or too old to know the endpoint — callers then fall back to
// the legacy token QR. Retries briefly so a just-launched serve has time to bind.
func mintEnrollCode(port int, token string) string {
	if port == 0 || token == "" {
		return ""
	}
	urlStr := fmt.Sprintf("http://127.0.0.1:%d/api/enroll/mint", port)
	client := &http.Client{Timeout: 2 * time.Second}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader([]byte("{}")))
		if err != nil {
			return ""
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var r struct {
					EnrollCode string `json:"enrollCode"`
				}
				if json.NewDecoder(resp.Body).Decode(&r) == nil && r.EnrollCode != "" {
					return r.EnrollCode
				}
			}
			return "" // reachable but no code (old serve) → don't retry
		}
		time.Sleep(500 * time.Millisecond) // not up yet — give it a moment
	}
	return ""
}

// printTunnelPairing prints the pairing block plus a foreground footer.
// stable=true for hosted (pair once), false for quick (the URL rotates each run).
func printTunnelPairing(url, token, name string, port int, stable bool) {
	printPairingBlock(url, token, name, port)
	if stable {
		i18n.Say(i18n.Dim+"Stable address — pair once; it stays the same across restarts. Keep this open; Ctrl-C stops it. Anyone with this URL + token can read your radar."+i18n.Reset,
			i18n.Dim+"固定地址：配一次即可，重启也不变。保持开启；Ctrl-C 关闭。拿到此 URL + token 的人都能读你的雷达。"+i18n.Reset)
	} else {
		i18n.Say(i18n.Dim+"Quick tunnel: the URL changes each run (use `gtmux tunnel` for a stable address). Keep this open; Ctrl-C stops it."+i18n.Reset,
			i18n.Dim+"Quick tunnel：每次运行地址都会变（想要固定地址用 `gtmux tunnel`）。保持开启；Ctrl-C 关闭。"+i18n.Reset)
	}
}

func tunnelUsage() {
	i18n.Say("usage: gtmux tunnel [--backend cloudflare|self] [--quick] [--service|--unservice|--status] [--port N] [--name LABEL]",
		"用法：gtmux tunnel [--backend cloudflare|self] [--quick] [--service|--unservice|--status] [--port N] [--name 标签]")
	i18n.Say("  Expose the read-only radar from anywhere via an outbound tunnel, then print a pairing QR.",
		"  通过出站隧道把只读雷达暴露到任何地方，并打印配对二维码。")
	i18n.Say("  default: a stable hosted address (pair once), foreground. --quick: an account-less ephemeral URL.",
		"  默认：固定的托管地址（配一次即可），前台运行。--quick：免账号的临时地址。")
	i18n.Say("  --backend self (\"Direct\"): tunnel over 443 (survives networks that block the hosted edge).",
		"  --backend self（即 \"Direct\"）：走 443 隧道（在屏蔽托管边缘的网络下仍可用）。")
	i18n.Say("  --redeem <code>: unlock Direct with an access code (or run your OWN server via",
		"  --redeem <码>：用访问码解锁 Direct（或用")
	i18n.Say("    GTMUX_SELFTUNNEL_URL + GTMUX_SELFTUNNEL_SECRET — see deploy/self-tunnel/README.md).",
		"    GTMUX_SELFTUNNEL_URL + GTMUX_SELFTUNNEL_SECRET 指向你自己的服务器 —— 见 deploy/self-tunnel/README.md）。")
	i18n.Say("  --service: keep it ON across reboots (launchd); --unservice: turn off; --status: show state.",
		"  --service：重启也保持开启（launchd）；--unservice：关闭；--status：查看状态。")
}

func tunnelUsageErr() int {
	tunnelUsage()
	return 2
}
