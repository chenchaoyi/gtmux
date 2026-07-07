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

// selfTunnelConfig resolves the "Direct" server: env > user conf > the baked-in
// gtmux-provided default. A user's own settings must win over the baked one.
func TestSelfTunnelConfigBakedFallback(t *testing.T) {
	t.Setenv("GTMUX_SELFTUNNEL_URL", "")
	t.Setenv("GTMUX_SELFTUNNEL_SECRET", "")
	t.Setenv("HOME", t.TempDir()) // no ~/.config/gtmux/selftunnel.conf there

	oldURL, oldSecret := SelfTunnelURL, SelfTunnelSecret
	defer func() { SelfTunnelURL, SelfTunnelSecret = oldURL, oldSecret }()
	SelfTunnelURL, SelfTunnelSecret = "https://direct.example.com", "user:pass"

	url, secret, ok := selfTunnelConfig()
	if !ok || url != "https://direct.example.com" || secret != "user:pass" {
		t.Fatalf("baked fallback: ok=%v url=%q secret=%q", ok, url, secret)
	}

	// The user's own env must OVERRIDE the baked default.
	t.Setenv("GTMUX_SELFTUNNEL_URL", "https://mine.example.com")
	t.Setenv("GTMUX_SELFTUNNEL_SECRET", "me:secret")
	if url, _, _ := selfTunnelConfig(); url != "https://mine.example.com" {
		t.Errorf("env must override baked default, got %q", url)
	}
}

// With no env, no conf, and no baked default, Direct is unconfigured → ok=false.
func TestSelfTunnelConfigUnconfigured(t *testing.T) {
	t.Setenv("GTMUX_SELFTUNNEL_URL", "")
	t.Setenv("GTMUX_SELFTUNNEL_SECRET", "")
	t.Setenv("HOME", t.TempDir())
	oldURL, oldSecret := SelfTunnelURL, SelfTunnelSecret
	defer func() { SelfTunnelURL, SelfTunnelSecret = oldURL, oldSecret }()
	SelfTunnelURL, SelfTunnelSecret = "", ""
	if _, _, ok := selfTunnelConfig(); ok {
		t.Error("unconfigured Direct should return ok=false")
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
