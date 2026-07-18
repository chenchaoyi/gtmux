// Package agentenv decides the proxy environment a coding-agent process gets at
// LAUNCH. Some networks reach the model API directly; others need an HTTP proxy.
// Which is which cannot be told apart reliably from inside the tool, so the choice
// is EXPLICIT and generic: gtmux applies a proxy ONLY when one is configured, and
// never probes or guesses. What proxy (if any) each network needs is the user's to
// configure — gtmux hard-codes nothing about any particular proxy tool or network.
//
// Resolved in order, first non-empty wins: the GTMUX_AGENT_PROXY env var, then
// agentProxy in ~/.config/gtmux/config.json, else none. A value is either an HTTP(S)
// proxy URL to apply, or "off" (equivalently empty) for no proxy. Set it with
// `gtmux config agent-proxy <url>|off`; the env var overrides for a per-network switch.
package agentenv

import (
	"fmt"
	"os"
	"strings"

	"github.com/chenchaoyi/gtmux/internal/usercfg"
)

type config struct {
	AgentProxy *string `json:"agentProxy"`
}

func loadConfig() config {
	var c config
	_ = usercfg.Load(&c)
	return c
}

// value resolves the configured proxy value: env override, then config, else "".
func value() string {
	if v := strings.TrimSpace(os.Getenv("GTMUX_AGENT_PROXY")); v != "" {
		return v
	}
	if c := loadConfig(); c.AgentProxy != nil {
		return strings.TrimSpace(*c.AgentProxy)
	}
	return ""
}

// proxyURL resolves the proxy to apply ("" = none) — EXPLICIT, never probed. A value
// that is a URL is applied; "off"/empty/anything-not-a-URL means no proxy.
func proxyURL() string {
	if v := value(); strings.Contains(v, "://") {
		return v
	}
	return ""
}

// Prefix returns the shell env prefix to prepend to an agent launch command
// ("" = nothing needed), e.g. `HTTPS_PROXY=<url> HTTP_PROXY=<url> `.
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
