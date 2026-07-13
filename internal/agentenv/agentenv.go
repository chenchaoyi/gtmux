// Package agentenv decides the proxy environment a coding-agent process needs at
// LAUNCH. Some networks reach the model API DIRECTLY (a plain office intranet, or
// Clash in transparent TUN mode) and need NO proxy; others (a double-VPN at home)
// block the direct path and need the local proxy. These CANNOT be told apart by any
// reliable local probe — the proxy port listens either way, a direct request 403s
// either way, and traffic routes through a utun either way — so the choice is
// EXPLICIT. gtmux never guesses (the old "auto" port-probe is removed: under Clash
// TUN 7897 still listens, which wrongly proxied an office launch).
//
// Resolved in order: the GTMUX_AGENT_PROXY env var, then agentProxy in
// ~/.config/gtmux/config.json, else "off". Values:
//
//	"off"               → no proxy (DEFAULT; office / TUN — launch bare)
//	"on"                → http://127.0.0.1:<agentProxyPort, default 7897> (home VPN)
//	"http://host:port"  → that explicit URL
//
// Set it per network with `gtmux config agent-proxy off|on|<url>` (writes the config)
// or the GTMUX_AGENT_PROXY env var (wire it to your network switch).
package agentenv

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultProxyPort is the local proxy port `on` uses when none is configured.
const DefaultProxyPort = 7897

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

// mode resolves the configured proxy mode: env override, then config, else "off".
func mode(c config) string {
	if v := strings.TrimSpace(os.Getenv("GTMUX_AGENT_PROXY")); v != "" {
		return v
	}
	if c.AgentProxy != nil {
		return strings.TrimSpace(*c.AgentProxy)
	}
	return "off"
}

// proxyURL resolves the proxy to apply ("" = none) — EXPLICIT, never probed.
//   - "on"              → http://127.0.0.1:<port>
//   - an explicit URL   → that URL
//   - "off" / unset / anything else (incl. the removed "auto") → "" (no proxy)
func proxyURL() string {
	c := loadConfig()
	switch m := mode(c); {
	case m == "on":
		return fmt.Sprintf("http://127.0.0.1:%d", proxyPort(c))
	case strings.Contains(m, "://"):
		return m
	default:
		return ""
	}
}

func proxyPort(c config) int {
	if c.AgentProxyPort != nil && *c.AgentProxyPort > 0 {
		return *c.AgentProxyPort
	}
	return DefaultProxyPort
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
// whether the launch is proxied.
func Active() string { return proxyURL() }
