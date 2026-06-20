package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/i18n"
	"github.com/chenchaoyi/gtmux/internal/server"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

const defaultServePort = 8765

// cmdServe implements `gtmux serve` — a local, read-only HTTP server that
// exposes the agent radar to the remote mobile app over a VPN/tunnel.
//
// It binds an intranet/VPN interface (default 0.0.0.0 so the phone can reach the
// Mac's internal IP), guards every /api/* route with a Bearer token, and serves
// ONLY read-only data plus a local "focus" (no input injection / no RCE).
func cmdServe(args []string) int {
	port := defaultServePort
	bind := "0.0.0.0"
	token := ""

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
			usage()
			return 0
		case a == "--port", a == "-p":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 || n > 65535 {
				i18n.Sae("gtmux serve: invalid --port", "gtmux serve: 无效的 --port")
				return 2
			}
			port = n
		case strings.HasPrefix(a, "--port="):
			n, err := strconv.Atoi(strings.TrimPrefix(a, "--port="))
			if err != nil || n <= 0 || n > 65535 {
				i18n.Sae("gtmux serve: invalid --port", "gtmux serve: 无效的 --port")
				return 2
			}
			port = n
		case a == "--bind":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			bind = v
		case strings.HasPrefix(a, "--bind="):
			bind = strings.TrimPrefix(a, "--bind=")
		case a == "--token":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			token = v
		case strings.HasPrefix(a, "--token="):
			token = strings.TrimPrefix(a, "--token=")
		default:
			i18n.Sae("gtmux serve: unknown option '"+a+"'", "gtmux serve: 未知选项 '"+a+"'")
			return 2
		}
	}

	token = resolveServeToken(token)
	addr := net.JoinHostPort(bind, strconv.Itoa(port))

	deps := server.Deps{
		AgentsJSON: func() ([]byte, error) {
			if !tmux.ServerUp() { // no tmux → empty array, same as `agents --json`
				return []byte("[]"), nil
			}
			return agentsJSONBytes()
		},
		PaneText: func(id string) (string, bool) {
			if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
				return "", false
			}
			return tmux.CapturePane(id), true
		},
		Focus: func(id string) error { return focusPaneByID(id) },
		AgentStatuses: func() []server.AgentStatus {
			if !tmux.ServerUp() {
				return nil
			}
			panes := gatherAgents()
			out := make([]server.AgentStatus, 0, len(panes))
			for _, p := range panes {
				out = append(out, server.AgentStatus{
					PaneID: p.paneID, Agent: p.agent, Loc: p.loc, Task: p.task, Status: p.status,
				})
			}
			return out
		},
	}

	srv := server.New(server.Config{Addr: addr, Token: token}, deps)
	printServeBanner(bind, port, token)
	if err := srv.ListenAndServe(); err != nil {
		i18n.Sae("gtmux serve: "+err.Error(), "gtmux serve: "+err.Error())
		return 1
	}
	return 0
}

func serveUsageErr() int {
	i18n.Sae("usage: gtmux serve [--port N] [--bind ADDR] [--token TOKEN]",
		"用法: gtmux serve [--port N] [--bind ADDR] [--token TOKEN]")
	return 2
}

// resolveServeToken returns the explicit flag token, or a persistent one from
// ~/.config/gtmux/serve-token (generating + writing it 0600 on first run).
func resolveServeToken(flagToken string) string {
	if flagToken != "" {
		return flagToken
	}
	path := filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "serve-token")
	if b, err := os.ReadFile(path); err == nil {
		if tok := strings.TrimSpace(string(b)); tok != "" {
			return tok
		}
	}
	tok := randToken()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		_ = os.WriteFile(path, []byte(tok+"\n"), 0o600)
	}
	return tok
}

// randToken returns 16 random bytes hex-encoded (128 bits).
func randToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is catastrophic; fail closed with a marker the
		// operator will notice rather than a silent weak token.
		return "INSECURE-RANDOM-FAILED"
	}
	return hex.EncodeToString(b)
}

// printServeBanner tells the user where to point the phone and the token to use.
func printServeBanner(bind string, port int, token string) {
	i18n.Say("gtmux serve — read-only remote radar (keep this behind a VPN/tunnel)",
		"gtmux serve — 只读远程雷达(请放在 VPN/隧道之后)")
	fmt.Printf("  token: %s\n", token)
	for _, host := range reachableHosts(bind) {
		fmt.Printf("  http://%s/api/agents\n", net.JoinHostPort(host, strconv.Itoa(port)))
	}
	i18n.Say("  test: curl -H \"Authorization: Bearer <token>\" <url>",
		"  自测: curl -H \"Authorization: Bearer <token>\" <url>")
}

// reachableHosts lists candidate hosts to advertise. A specific --bind is
// returned as-is; a wildcard bind expands to the non-loopback IPv4 interface
// addresses (the intranet/VPN IPs the phone routes to).
func reachableHosts(bind string) []string {
	if bind != "" && bind != "0.0.0.0" && bind != "::" {
		return []string{bind}
	}
	var hosts []string
	addrs, _ := net.InterfaceAddrs()
	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			hosts = append(hosts, ipnet.IP.String())
		}
	}
	if len(hosts) == 0 {
		hosts = []string{"localhost"}
	}
	return hosts
}
