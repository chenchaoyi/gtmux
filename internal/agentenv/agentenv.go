// Package agentenv decides the environment a coding-agent process needs at
// LAUNCH so the user never hand-toggles a proxy when switching networks.
//
// The motivating case: at home behind a double-VPN, a launched `claude` reaches
// the model API only through the local proxy (Clash on 127.0.0.1:7897) or it
// 403s; on a plain office intranet it must NOT be proxied. Rather than make the
// user edit config per network, gtmux detects the context: "auto" applies the
// proxy IFF that port is listening (i.e. the proxy tool is running — the home
// case), and adds nothing when it isn't (the intranet case).
//
// Configured in ~/.config/gtmux/config.json:
//
//	"agentProxy":     "auto" | "http://host:port" | "off"   (default "auto")
//	"agentProxyPort": 7897                                   ("auto" probe port)
package agentenv

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const defaultProxyPort = 7897

type config struct {
	AgentProxy     *string `json:"agentProxy"`
	AgentProxyPort *int    `json:"agentProxyPort"`
}

func loadConfig() config {
	var c config
	b, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "config.json"))
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

// proxyURL resolves the proxy to use ("" = none), per config + detection.
//   - "off"            → ""
//   - explicit URL     → that URL
//   - "auto" (default) → http://127.0.0.1:<port> IFF that port is listening
func proxyURL() string {
	c := loadConfig()
	mode := "auto"
	if c.AgentProxy != nil {
		mode = strings.TrimSpace(*c.AgentProxy)
	}
	switch {
	case mode == "off" || mode == "":
		return ""
	case mode != "auto":
		return mode // an explicit proxy URL
	}
	port := defaultProxyPort
	if c.AgentProxyPort != nil && *c.AgentProxyPort > 0 {
		port = *c.AgentProxyPort
	}
	if portListening(port) {
		return fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	return ""
}

// portListening reports whether something accepts TCP on 127.0.0.1:port (a fast
// probe — the "is the proxy tool running" signal).
func portListening(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Prefix returns the shell env prefix to prepend to an agent launch command
// ("" = nothing needed), e.g. `HTTPS_PROXY=http://127.0.0.1:7897 HTTP_PROXY=http://127.0.0.1:7897 `.
func Prefix() string {
	u := proxyURL()
	if u == "" {
		return ""
	}
	return fmt.Sprintf("HTTPS_PROXY=%s HTTP_PROXY=%s ", u, u)
}

// Wrap prepends Prefix() to command, UNLESS the command already sets a proxy
// (the user hand-wrote one) — so we never double it.
func Wrap(command string) string {
	if strings.Contains(command, "HTTPS_PROXY=") || strings.Contains(command, "HTTP_PROXY=") {
		return command
	}
	return Prefix() + command
}

// Active returns the proxy URL a launch WOULD apply on this network ("" = none) —
// the resolved value the `gtmux spawn` pre-flight reports so a dispatcher can see
// whether the launch is proxied (a bare, un-proxied launch is the incident-① 403).
func Active() string { return proxyURL() }
