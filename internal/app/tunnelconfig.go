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
	// NB: the "Direct" tunnel server + its chisel secret are deliberately NOT baked
	// into the binary. Direct is gtmux's paid tunnel — the config is delivered by the
	// control-plane Worker only on a valid access code (`gtmux tunnel --redeem`), which
	// writes ~/.config/gtmux/selftunnel.conf. Keeping the secret out of the (public)
	// binary is what lets the repo stay fully open-source.
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
