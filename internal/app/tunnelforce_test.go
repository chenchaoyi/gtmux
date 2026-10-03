package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProvisionForceRequiresRepairReceipt(t *testing.T) {
	for _, tt := range []struct {
		name     string
		force    bool
		repaired bool
		wantErr  bool
	}{
		{"ordinary request remains compatible", false, false, false},
		{"repair acknowledged", true, true, false},
		{"older worker cannot pretend to repair", true, false, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if r.URL.Path != "/provision" || r.Header.Get("x-gtmux-reg") != "test-gate" || body["deviceId"] != "unchanged-device-0001" {
					t.Errorf("unexpected provision request: %s %+v", r.URL.Path, body)
				}
				if tt.force && body["force"] != true {
					t.Error("repair flag was not forwarded")
				}
				if !tt.force {
					if _, present := body["force"]; present {
						t.Error("ordinary request must omit force for older workers")
					}
				}
				_ = json.NewEncoder(w).Encode(provisionResp{URL: "https://stable.example", Token: "connector", Repaired: tt.repaired})
			}))
			defer srv.Close()
			_, retry, err := provisionOnce(srv.URL, "test-gate", "unchanged-device-0001", "Mac", tt.force)
			if (err != nil) != tt.wantErr || retry {
				t.Fatalf("error=%v retry=%v", err, retry)
			}
			if tt.wantErr && !strings.Contains(err.Error(), "did not acknowledge") {
				t.Fatalf("missing compatibility explanation: %v", err)
			}
		})
	}
}

func TestForceRepairFailureKeepsInstalledServiceAndDeviceID(t *testing.T) {
	setDeviceID(t, "unchanged-device-0001")
	home := os.Getenv("HOME")
	bin := t.TempDir()
	if err := os.WriteFile(filepath.Join(bin, "cloudflared"), []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	t.Setenv("GTMUX_TUNNEL_REG", "test-gate")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate an older worker ignoring force. The local agents must not be
		// replaced or restarted merely because it returns a valid token.
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["force"] != true || body["deviceId"] != "unchanged-device-0001" {
			t.Errorf("service repair did not forward force with the original ID: %+v %v", body, err)
		}
		_ = json.NewEncoder(w).Encode(provisionResp{URL: "https://stable.example", Token: "connector"})
	}))
	defer srv.Close()
	t.Setenv("GTMUX_TUNNEL_API", srv.URL)
	t.Setenv("GTMUX_TUNNEL_API_FALLBACK", srv.URL)
	files := []string{serveAgentPath(), tunnelAgentPath(), tunnelURLPath()}
	for _, p := range files {
		if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("original\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if rc := cmdTunnel([]string{"--backend", "cloudflare", "--service", "--force", "--yes"}); rc != 1 {
		t.Fatalf("exit=%d, want failure", rc)
	}
	for _, p := range files {
		got, err := os.ReadFile(p)
		if err != nil || string(got) != "original\n" {
			t.Fatalf("repair changed %s: %q %v", p, got, err)
		}
	}
	got, err := os.ReadFile(filepath.Join(home, ".config", "gtmux", "tunnel-device-id"))
	if err != nil || string(got) != "unchanged-device-0001\n" {
		t.Fatalf("repair rotated identity: %q %v", got, err)
	}
}

func TestForceRejectsOtherTunnelOperations(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("GTMUX_TUNNEL_BACKEND", "")
	for _, args := range [][]string{
		{"--force"}, {"--force", "--status"}, {"--force", "--unservice"},
		{"--force", "--service", "--quick"},
		{"--force", "--service", "--backend", "self"},
		{"--force", "--service", "--redeem", "code"},
		{"--force", "--service", "--servers"},
	} {
		if rc := cmdTunnel(args); rc != 2 {
			t.Fatalf("%v: exit=%d", args, rc)
		}
	}
}
