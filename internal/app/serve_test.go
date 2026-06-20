package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveServeTokenFlagWins(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := resolveServeToken("explicit"); got != "explicit" {
		t.Fatalf("flag token = %q, want explicit", got)
	}
	// An explicit token must not write the persistent file.
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), ".config", "gtmux", "serve-token")); !os.IsNotExist(err) {
		t.Fatalf("explicit token should not persist a file")
	}
}

func TestResolveServeTokenGeneratesAndPersists(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	first := resolveServeToken("")
	if len(first) < 16 {
		t.Fatalf("generated token too short: %q", first)
	}
	path := filepath.Join(home, ".config", "gtmux", "serve-token")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("token file not written: %v", err)
	}
	if perm := fi.Mode().Perm(); perm != 0o600 {
		t.Fatalf("token file perm = %o, want 600", perm)
	}

	// A second call reuses the persisted token (stable across restarts).
	if second := resolveServeToken(""); second != first {
		t.Fatalf("token not stable: %q != %q", second, first)
	}
}

func TestReachableHostsSpecificBind(t *testing.T) {
	hosts := reachableHosts("10.0.0.5")
	if len(hosts) != 1 || hosts[0] != "10.0.0.5" {
		t.Fatalf("specific bind hosts = %v, want [10.0.0.5]", hosts)
	}
}

func TestReachableHostsWildcardNonEmpty(t *testing.T) {
	// Wildcard expands to interface IPs; on any host there is at least the
	// fallback, so the slice is never empty (avoids an empty banner).
	if hosts := reachableHosts("0.0.0.0"); len(hosts) == 0 {
		t.Fatalf("wildcard hosts must not be empty")
	}
}
