package app

import "testing"

// resolveRelay: env GTMUX_RELAY_URL / GTMUX_RELAY_TOKEN override the baked
// defaults; with neither set, the baked values are returned.
func TestResolveRelayBakedDefaults(t *testing.T) {
	t.Setenv("GTMUX_RELAY_URL", "")
	t.Setenv("GTMUX_RELAY_TOKEN", "")
	url, token := resolveRelay()
	if url != RelayURL {
		t.Errorf("url = %q, want baked default %q", url, RelayURL)
	}
	if token != RelayToken {
		t.Errorf("token = %q, want baked default %q", token, RelayToken)
	}
	if url == "" {
		t.Errorf("baked RelayURL must not be empty")
	}
}

func TestResolveRelayURLOverride(t *testing.T) {
	t.Setenv("GTMUX_RELAY_URL", "https://relay.test/push")
	t.Setenv("GTMUX_RELAY_TOKEN", "")
	url, token := resolveRelay()
	if url != "https://relay.test/push" {
		t.Errorf("url = %q, want the env override", url)
	}
	if token != RelayToken {
		t.Errorf("token = %q, want baked default %q when only URL overridden", token, RelayToken)
	}
}

func TestResolveRelayTokenOverride(t *testing.T) {
	t.Setenv("GTMUX_RELAY_URL", "")
	t.Setenv("GTMUX_RELAY_TOKEN", "secret-token")
	url, token := resolveRelay()
	if token != "secret-token" {
		t.Errorf("token = %q, want the env override", token)
	}
	if url != RelayURL {
		t.Errorf("url = %q, want baked default %q when only token overridden", url, RelayURL)
	}
}

func TestResolveRelayBothOverride(t *testing.T) {
	t.Setenv("GTMUX_RELAY_URL", "https://r.test")
	t.Setenv("GTMUX_RELAY_TOKEN", "tok")
	url, token := resolveRelay()
	if url != "https://r.test" || token != "tok" {
		t.Errorf("resolveRelay() = (%q, %q), want (https://r.test, tok)", url, token)
	}
}
