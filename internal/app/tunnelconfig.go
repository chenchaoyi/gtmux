package app

import "os"

// Build-time configuration for hosted tunnels (the A1 control plane).
//
// TunnelAPI points at gtmux's hosted control-plane Worker. TunnelRegSecret is a
// soft anti-abuse gate that necessarily ships in the binary (it is NOT a real
// secret) — left empty in source and injected at release build via
// -ldflags "-X github.com/chenchaoyi/gtmux/internal/app.TunnelRegSecret=<v>"
// from a CI secret. Both are overridable at runtime via GTMUX_TUNNEL_API /
// GTMUX_TUNNEL_REG (handy for self-hosters and local testing).
var (
	TunnelAPI = "https://api.gtmux.ccy.dev"
	// TunnelAPIFallback is a second base the provision call tries when the
	// primary is unreachable (e.g. a flaky network resets the custom domain) —
	// the same Worker via its workers.dev route.
	TunnelAPIFallback = "https://gtmux-tunnel.ccy-chenchaoyi.workers.dev"
	TunnelRegSecret   = ""
	// SelfTunnelURL / SelfTunnelSecret bake gtmux's "Direct" tunnel server (a chisel
	// endpoint on a gtmux-run VPS + TLS reverse proxy) so a user can pick Direct with
	// ZERO config — it's the second of the two gtmux-provided tunnels ("Standard" =
	// Cloudflare, "Direct" = this). Like TunnelRegSecret they necessarily ship in the
	// binary; empty in source, injected at release build via -ldflags from
	// GTMUX_SELFTUNNEL_URL / GTMUX_SELFTUNNEL_SECRET, and overridable at runtime by
	// those env vars or ~/.config/gtmux/selftunnel.conf (a user's own server wins).
	SelfTunnelURL    = ""
	SelfTunnelSecret = ""
)

func tunnelAPI() string {
	if v := os.Getenv("GTMUX_TUNNEL_API"); v != "" {
		return v
	}
	return TunnelAPI
}

func tunnelAPIFallback() string {
	if v := os.Getenv("GTMUX_TUNNEL_API_FALLBACK"); v != "" {
		return v
	}
	return TunnelAPIFallback
}

func tunnelRegSecret() string {
	if v := os.Getenv("GTMUX_TUNNEL_REG"); v != "" {
		return v
	}
	return TunnelRegSecret
}
