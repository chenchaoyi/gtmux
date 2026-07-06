package app

import "testing"

// tunnelAPI / tunnelAPIFallback / tunnelRegSecret: env override wins; otherwise
// the baked package default (or empty for the secret) is returned.
func TestTunnelAPIEnvOverride(t *testing.T) {
	t.Setenv("GTMUX_TUNNEL_API", "https://example.test/api")
	if got := tunnelAPI(); got != "https://example.test/api" {
		t.Fatalf("tunnelAPI() = %q, want the env override", got)
	}
}

func TestTunnelAPIBakedDefault(t *testing.T) {
	t.Setenv("GTMUX_TUNNEL_API", "")
	if got := tunnelAPI(); got != TunnelAPI {
		t.Fatalf("tunnelAPI() = %q, want baked default %q", got, TunnelAPI)
	}
	if got := tunnelAPI(); got == "" {
		t.Fatalf("baked TunnelAPI must not be empty")
	}
}

func TestTunnelAPIFallbackEnvOverride(t *testing.T) {
	t.Setenv("GTMUX_TUNNEL_API_FALLBACK", "https://fallback.test")
	if got := tunnelAPIFallback(); got != "https://fallback.test" {
		t.Fatalf("tunnelAPIFallback() = %q, want the env override", got)
	}
}

func TestTunnelAPIFallbackBakedDefault(t *testing.T) {
	t.Setenv("GTMUX_TUNNEL_API_FALLBACK", "")
	if got := tunnelAPIFallback(); got != TunnelAPIFallback {
		t.Fatalf("tunnelAPIFallback() = %q, want baked default %q", got, TunnelAPIFallback)
	}
}

func TestTunnelRegSecretEnvOverride(t *testing.T) {
	t.Setenv("GTMUX_TUNNEL_REG", "soft-gate-123")
	if got := tunnelRegSecret(); got != "soft-gate-123" {
		t.Fatalf("tunnelRegSecret() = %q, want the env override", got)
	}
}

func TestTunnelRegSecretFallsBackToBaked(t *testing.T) {
	t.Setenv("GTMUX_TUNNEL_REG", "")
	// In source the baked secret is empty (injected only at release build), so the
	// fallback is whatever TunnelRegSecret holds — equal by construction.
	if got := tunnelRegSecret(); got != TunnelRegSecret {
		t.Fatalf("tunnelRegSecret() = %q, want baked default %q", got, TunnelRegSecret)
	}
}

// cloudflaredProtocol: default http2 (QUIC is blocked on many corp nets), env override wins.
func TestCloudflaredProtocolDefault(t *testing.T) {
	t.Setenv("GTMUX_TUNNEL_PROTOCOL", "")
	if got := cloudflaredProtocol(); got != "http2" {
		t.Fatalf("cloudflaredProtocol() = %q, want default http2", got)
	}
}

func TestCloudflaredProtocolEnvOverride(t *testing.T) {
	t.Setenv("GTMUX_TUNNEL_PROTOCOL", "quic")
	if got := cloudflaredProtocol(); got != "quic" {
		t.Fatalf("cloudflaredProtocol() = %q, want the env override quic", got)
	}
	t.Setenv("GTMUX_TUNNEL_PROTOCOL", "  auto  ")
	if got := cloudflaredProtocol(); got != "auto" {
		t.Fatalf("cloudflaredProtocol() = %q, want trimmed auto", got)
	}
}
