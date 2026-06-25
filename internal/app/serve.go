package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
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
	relayURL := ""
	relayToken := ""

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
		case a == "--relay-url":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			relayURL = v
		case strings.HasPrefix(a, "--relay-url="):
			relayURL = strings.TrimPrefix(a, "--relay-url=")
		case a == "--relay-token":
			v, ok := next()
			if !ok {
				return serveUsageErr()
			}
			relayToken = v
		case strings.HasPrefix(a, "--relay-token="):
			relayToken = strings.TrimPrefix(a, "--relay-token=")
		default:
			i18n.Sae("gtmux serve: unknown option '"+a+"'", "gtmux serve: 未知选项 '"+a+"'")
			return 2
		}
	}

	token = resolveServeToken(token)
	srv := newServeServer(bind, port, token, relayURL, relayToken)
	printServeBanner(bind, port, token)
	if err := srv.ListenAndServe(); err != nil {
		i18n.Sae("gtmux serve: "+err.Error(), "gtmux serve: "+err.Error())
		return 1
	}
	return 0
}

// newServeServer builds the read-only radar HTTP server (shared by `gtmux serve`
// and `gtmux tunnel`, which starts it in-process when one isn't already up).
func newServeServer(bind string, port int, token, relayURL, relayToken string) *server.Server {
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
			return tmux.CapturePaneColor(id), true
		},
		Focus:  func(id string) error { return focusPaneByID(id) },
		Send:   sendToPane,
		Upload: saveUpload,
		Icon:   agentIconPNG,
		Diff:   diffForPane,
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

	// Push: tokens live here (the relay stays stateless); alerts are forwarded
	// out to the relay, which holds the APNs key. With no explicit --relay-url
	// (e.g. the serve that `gtmux tunnel` / always-on starts), fall back to the
	// hosted relay so notifications work by default.
	if relayURL == "" {
		relayURL, relayToken = resolveRelay()
	}
	relay := server.NewHTTPRelay(relayURL, relayToken)
	deps.Push = server.NewPushManager(relay, loadPushTokens(), savePushTokens, pushCopy)

	// Per-device enrollment: a phone pairs via a short-lived code for its own
	// token (revocable, and the QR stops being a lasting credential). The master
	// token keeps working, so existing pairings are unaffected.
	deps.Enroll = server.NewEnrollManager(loadDevices(), saveDevices)

	return server.New(server.Config{Addr: addr, Token: token}, deps)
}

func serveUsageErr() int {
	i18n.Sae("usage: gtmux serve [--port N] [--bind ADDR] [--token TOKEN] [--relay-url URL] [--relay-token TOKEN]",
		"用法：gtmux serve [--port N] [--bind ADDR] [--token TOKEN] [--relay-url URL] [--relay-token TOKEN]")
	return 2
}

// pushTokensPath is where registered device push tokens persist across restarts.
func pushTokensPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "push-tokens.json")
}

func loadPushTokens() []server.DeviceToken {
	b, err := os.ReadFile(pushTokensPath())
	if err != nil {
		return nil
	}
	var out []server.DeviceToken
	_ = json.Unmarshal(b, &out)
	return out
}

func savePushTokens(toks []server.DeviceToken) {
	path := pushTokensPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if b, err := json.Marshal(toks); err == nil {
		_ = os.WriteFile(path, b, 0o600)
	}
}

// devicesPath is where the enrolled-device roster persists (per-device tokens).
func devicesPath() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "devices.json")
}

func loadDevices() []server.EnrolledDevice {
	b, err := os.ReadFile(devicesPath())
	if err != nil {
		return nil
	}
	var out []server.EnrolledDevice
	_ = json.Unmarshal(b, &out)
	return out
}

func saveDevices(d []server.EnrolledDevice) {
	path := devicesPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return
	}
	if b, err := json.Marshal(d); err == nil {
		_ = os.WriteFile(path, b, 0o600)
	}
}

// pushCopy renders an alert into a bilingual notification title/body
// (en/zh via GTMUX_LANG). The body is the agent's current task.
func pushCopy(a server.Alert) (string, string) {
	name := a.Agent
	if name == "" {
		name = i18n.Tr("agent", "agent")
	}
	if a.Kind == "waiting" {
		if a.Repeat {
			return fmt.Sprintf(i18n.Tr("%s still needs you", "%s 仍在等你"), name), a.Task
		}
		return fmt.Sprintf(i18n.Tr("%s needs you", "%s 等你输入"), name), a.Task
	}
	return fmt.Sprintf(i18n.Tr("%s finished", "%s 完成了"), name), a.Task
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

// allowedSendKeys whitelists the named control keys POST /api/send accepts (a
// tmux key name must never be free-form — the rest goes through as literal text).
var allowedSendKeys = map[string]bool{
	"Enter": true, "C-c": true, "Escape": true, "Tab": true,
	"Up": true, "Down": true, "Left": true, "Right": true,
	"BSpace": true, "C-d": true, "C-z": true, "C-l": true,
}

// maxDiffBytes caps the diff payload so a huge working tree can't bloat the phone
// (the rest is the agent's recent edits; a 400 KB head is plenty to review).
const maxDiffBytes = 400 << 10

// diffForPane returns a unified `git diff` (working tree vs HEAD) plus a list of
// untracked files for the pane's current working directory — "what did the agent
// change". Returns "" (no error) when the cwd isn't a git repo. Read-only; git
// runs with color disabled so the phone renders the raw unified diff.
func diffForPane(id string) (string, error) {
	if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
		return "", fmt.Errorf("pane not found")
	}
	cwd := tmux.Display(id, "#{pane_current_path}")
	if cwd == "" {
		return "", fmt.Errorf("pane not found")
	}
	git := func(args ...string) string {
		c := exec.Command("git", append([]string{"-C", cwd, "-c", "color.ui=never"}, args...)...)
		out, _ := c.Output()
		return string(out)
	}
	// Not a git repo → empty diff (the app shows a friendly note, not an error).
	c := exec.Command("git", "-C", cwd, "rev-parse", "--is-inside-work-tree")
	if c.Run() != nil {
		return "", nil
	}
	var b strings.Builder
	if branch := strings.TrimSpace(git("rev-parse", "--abbrev-ref", "HEAD")); branch != "" {
		fmt.Fprintf(&b, "# branch %s\n", branch)
	}
	b.WriteString(git("diff", "HEAD"))
	if u := strings.TrimSpace(git("ls-files", "--others", "--exclude-standard")); u != "" {
		b.WriteString("\n# untracked:\n")
		for _, f := range strings.Split(u, "\n") {
			fmt.Fprintf(&b, "+ %s\n", f)
		}
	}
	out := b.String()
	if len(out) > maxDiffBytes {
		out = out[:maxDiffBytes] + "\n… (diff truncated)\n"
	}
	return out, nil
}

// sendToPane types into a pane for POST /api/send (a WRITE). A non-empty key must
// be in the allowlist; otherwise text is typed literally (+ Enter when enter).
func sendToPane(id, text, key string, enter bool) error {
	if tmux.Bin == "" || tmux.Display(id, "#{pane_id}") == "" {
		return fmt.Errorf("pane not found")
	}
	if key != "" {
		if !allowedSendKeys[key] {
			return fmt.Errorf("key not allowed")
		}
		return tmux.SendKey(id, key)
	}
	return tmux.SendText(id, text, enter)
}

// saveUpload writes an uploaded file under ~/.local/share/gtmux/uploads with a
// random prefix (no collisions / overwrites) and returns its path, so the phone
// can hand a photo/file to an agent by path. Read by whoever the agent can read.
func saveUpload(name string, data []byte) (string, error) {
	dir := filepath.Join(os.Getenv("HOME"), ".local", "share", "gtmux", "uploads")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	safe := sanitizeFilename(name)
	if safe == "" {
		safe = "upload"
	}
	path := filepath.Join(dir, randToken()[:8]+"-"+safe)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// sanitizeFilename keeps just the base name with a conservative charset, so an
// uploaded name can't escape the uploads dir or inject anything.
func sanitizeFilename(name string) string {
	name = filepath.Base(name)
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '.', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.TrimLeft(b.String(), ".") // no leading dots
	if len(out) > 80 {
		out = out[len(out)-80:]
	}
	return out
}

// agentIconPNG returns a PNG of the agent's identity icon, extracted from its
// installed .app via sips (cached by app mtime), or nil. Read-only; uses the
// user's installed app — nothing third-party is bundled (DESIGN §6).
func agentIconPNG(name string) []byte {
	hint := iconFor(name, loadProfiles())
	if hint == "" {
		return nil
	}
	if !strings.HasSuffix(hint, ".app") {
		b, _ := os.ReadFile(hint) // a direct image path
		return b
	}
	cacheDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "gtmux", "icon-cache")
	_ = os.MkdirAll(cacheDir, 0o755)
	cache := filepath.Join(cacheDir, sanitizeFilename(name)+"-"+strconv.FormatInt(fileMtime(hint), 10)+".png")
	if b, err := os.ReadFile(cache); err == nil {
		return b
	}
	icns := appIcns(hint)
	if icns == "" {
		return nil
	}
	if exec.Command("sips", "-s", "format", "png", "-Z", "64", icns, "--out", cache).Run() != nil {
		return nil
	}
	b, _ := os.ReadFile(cache)
	return b
}

// appIcns finds a .app's .icns icon (CFBundleIconFile, else the first *.icns).
func appIcns(app string) string {
	res := filepath.Join(app, "Contents", "Resources")
	if out, err := exec.Command("defaults", "read", filepath.Join(app, "Contents", "Info"), "CFBundleIconFile").Output(); err == nil {
		name := strings.TrimSpace(string(out))
		if name != "" {
			if !strings.HasSuffix(name, ".icns") {
				name += ".icns"
			}
			if p := filepath.Join(res, name); fileExists(p) {
				return p
			}
		}
	}
	if m, _ := filepath.Glob(filepath.Join(res, "*.icns")); len(m) > 0 {
		return m[0]
	}
	return ""
}

// printServeBanner tells the user where to point the phone and the token to use.
func printServeBanner(bind string, port int, token string) {
	i18n.Say("gtmux serve — read-only remote radar (keep this behind a VPN/tunnel)",
		"gtmux serve — 只读远程雷达（请放在 VPN/隧道之后）")
	fmt.Printf("  token: %s\n", token)
	for _, host := range reachableHosts(bind) {
		fmt.Printf("  http://%s/api/agents\n", net.JoinHostPort(host, strconv.Itoa(port)))
	}
	i18n.Say("  test: curl -H \"Authorization: Bearer <token>\" <url>",
		"  自测：curl -H \"Authorization: Bearer <token>\" <url>")
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
