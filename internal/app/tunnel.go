package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	qrterminal "github.com/mdp/qrterminal/v3"
)

// cmdTunnel implements `gtmux tunnel` — expose the read-only radar to the phone
// from ANYWHERE (no LAN, no VPN app) by running an outbound reverse tunnel on the
// Mac. The tunnel client (cloudflared / frpc) lives only here; the phone app just
// gets a `{url, token}` pairing payload, so the transport never touches the app.
//
// Cloudflare is the default (no VPS, instant quick tunnel). frp is the China /
// own-VPS path and is documented rather than driven here for now.
func cmdTunnel(args []string) int {
	port := defaultServePort
	provider := "cloudflare"
	name, _ := os.Hostname()
	if name == "" {
		name = "Mac"
	}

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
		case a == "--provider":
			v, ok := next()
			if !ok {
				return tunnelUsageErr()
			}
			provider = v
		case strings.HasPrefix(a, "--provider="):
			provider = strings.TrimPrefix(a, "--provider=")
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

	switch provider {
	case "cloudflare", "cf":
		return tunnelCloudflare(port, name)
	case "frp":
		i18n.Say("frp isn't built into `gtmux tunnel` yet — see the remote-access docs for the frp setup (best for China / your own VPS). Cloudflare needs no VPS: `gtmux tunnel`.",
			"frp 暂未内置进 `gtmux tunnel` —— 搭建见远程访问文档(国内 / 自有 VPS 首选)。Cloudflare 无需 VPS:`gtmux tunnel`。")
		return 0
	default:
		i18n.Sae("gtmux tunnel: unknown provider '"+provider+"' (use: cloudflare | frp)",
			"gtmux tunnel: 未知 provider '"+provider+"'(可用:cloudflare | frp)")
		return 2
	}
}

// tryCloudflareRe matches the public URL cloudflared prints for a quick tunnel.
var tryCloudflareRe = regexp.MustCompile(`https://[a-z0-9-]+\.trycloudflare\.com`)

// tunnelCloudflare runs a Cloudflare quick tunnel (`cloudflared tunnel --url`)
// in front of the local read-only server, then prints the pairing URL + QR.
func tunnelCloudflare(port int, name string) int {
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
		if bin = ensureCloudflared(); bin == "" {
			return 1
		}
	}

	token := resolveServeToken("")

	// Make sure something read-only answers on the port. If `gtmux serve` is
	// already up we tunnel straight to it (its token matches — same file);
	// otherwise we start the radar in-process bound to loopback (the tunnel
	// reaches it locally, nothing else is exposed).
	if portInUse(port) {
		i18n.Say(fmt.Sprintf("Found a server on :%d — tunneling to it.", port),
			fmt.Sprintf("检测到 :%d 上已有服务 —— 直接给它开隧道。", port))
	} else {
		srv := newServeServer("127.0.0.1", port, token, "", "")
		go func() { _ = srv.ListenAndServe() }()
		for i := 0; i < 100 && !portInUse(port); i++ {
			time.Sleep(20 * time.Millisecond)
		}
		i18n.Say(fmt.Sprintf("Started the read-only radar on 127.0.0.1:%d.", port),
			fmt.Sprintf("已在 127.0.0.1:%d 启动只读雷达。", port))
	}

	i18n.Say("Opening a Cloudflare quick tunnel (no account, ephemeral URL)…",
		"正在打开 Cloudflare quick tunnel(免账号、临时地址)…")
	cmd := exec.Command(bin, "tunnel", "--no-autoupdate", "--url", fmt.Sprintf("http://localhost:%d", port))
	stderr, err := cmd.StderrPipe()
	if err != nil {
		i18n.Sae("gtmux tunnel: "+err.Error(), "gtmux tunnel: "+err.Error())
		return 1
	}
	if err := cmd.Start(); err != nil {
		i18n.Sae("gtmux tunnel: failed to start cloudflared: "+err.Error(),
			"gtmux tunnel: 启动 cloudflared 失败: "+err.Error())
		return 1
	}

	// Relay Ctrl-C to cloudflared so the tunnel shuts down cleanly.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		if _, ok := <-sigc; ok && cmd.Process != nil {
			_ = cmd.Process.Signal(syscall.SIGTERM)
		}
	}()

	// cloudflared logs to stderr, including the public URL. Echo its lines dimmed
	// and, on the first URL we see, print the pairing block + QR.
	printed := false
	sc := bufio.NewScanner(stderr)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		fmt.Fprintln(os.Stderr, i18n.Dim+line+i18n.Reset)
		if !printed {
			if m := tryCloudflareRe.FindString(line); m != "" {
				printTunnelPairing(m, token, name)
				printed = true
			}
		}
	}
	err = cmd.Wait()
	signal.Stop(sigc)
	close(sigc)
	if err != nil && !printed {
		i18n.Sae("gtmux tunnel: cloudflared exited: "+err.Error(),
			"gtmux tunnel: cloudflared 退出: "+err.Error())
		return 1
	}
	return 0
}

// ensureCloudflared offers to `brew install cloudflared` when it's missing.
// Returns the resolved path, or "" if the user declined / it couldn't be set up.
func ensureCloudflared() string {
	i18n.Say("cloudflared isn't installed — it's the Cloudflare tunnel client (one binary, Mac-side only; the phone app never touches it).",
		"未检测到 cloudflared —— 它是 Cloudflare 隧道客户端(一个二进制,只在 Mac 上跑;手机 app 完全不碰它)。")
	if _, err := exec.LookPath("brew"); err != nil {
		i18n.Say("Install it then re-run: https://github.com/cloudflare/cloudflared/releases",
			"请先安装再重试:https://github.com/cloudflare/cloudflared/releases")
		return ""
	}
	if !confirm(i18n.Tr("Install it now with `brew install cloudflared`? [Y/n] ",
		"现在用 `brew install cloudflared` 安装?[Y/n] ")) {
		i18n.Say("Skipped. Install it yourself, then re-run `gtmux tunnel`.",
			"已跳过。自行安装后重试 `gtmux tunnel`。")
		return ""
	}
	c := exec.Command("brew", "install", "cloudflared")
	c.Stdout, c.Stderr, c.Stdin = os.Stdout, os.Stderr, os.Stdin
	if err := c.Run(); err != nil {
		i18n.Sae("gtmux tunnel: brew install failed: "+err.Error(),
			"gtmux tunnel: brew 安装失败: "+err.Error())
		return ""
	}
	bin, err := exec.LookPath("cloudflared")
	if err != nil {
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

// printTunnelPairing prints the public URL, token, and a scannable pairing QR
// (the same `{v,url,token,name}` payload the menu-bar app encodes).
func printTunnelPairing(url, token, name string) {
	payload, _ := json.Marshal(struct {
		V     int    `json:"v"`
		URL   string `json:"url"`
		Token string `json:"token"`
		Name  string `json:"name"`
	}{1, url, token, name})

	fmt.Println()
	i18n.Say(i18n.Bold+"Your Mac is reachable from anywhere now:"+i18n.Reset,
		i18n.Bold+"你的 Mac 现在可从任何地方访问:"+i18n.Reset)
	fmt.Printf("  URL:   %s\n", url)
	fmt.Printf("  token: %s\n", token)
	fmt.Println()
	i18n.Say("Scan in the gtmux phone app (Pair → Scan):",
		"在 gtmux 手机 app 里扫码(配对 → 扫一扫):")
	qrterminal.GenerateHalfBlock(string(payload), qrterminal.L, os.Stdout)
	i18n.Say(i18n.Dim+"Quick tunnel: the URL changes each run. Keep this open; Ctrl-C stops it. Anyone with this URL + token can read your radar."+i18n.Reset,
		i18n.Dim+"Quick tunnel:每次运行地址都会变。保持开启;Ctrl-C 关闭。拿到此 URL + token 的人都能读你的雷达。"+i18n.Reset)
}

func tunnelUsage() {
	i18n.Say("usage: gtmux tunnel [--provider cloudflare|frp] [--port N] [--name LABEL]",
		"用法: gtmux tunnel [--provider cloudflare|frp] [--port N] [--name 标签]")
	i18n.Say("  Expose the read-only radar from anywhere via an outbound tunnel, then print a pairing QR.",
		"  通过出站隧道把只读雷达暴露到任何地方,并打印配对二维码。")
}

func tunnelUsageErr() int {
	tunnelUsage()
	return 2
}
