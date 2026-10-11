package app

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func standardRecoveryFixture(t *testing.T) (string, map[string][]byte) {
	t.Helper()
	setDeviceID(t, "previously-deleted-device-0001")
	if err := os.MkdirAll(launchAgentsDir(), 0700); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	log := filepath.Join(bin, "calls")
	// Always report loaded; any load/unload is captured, without touching launchd.
	script := "#!/bin/sh\n[ \"$1\" = list ] && exit 0\nprintf '%s %s\\n' \"$1\" \"$3\" >> \"" + log + "\"\n"
	if err := os.WriteFile(filepath.Join(bin, "launchctl"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":"+os.Getenv("PATH"))
	for _, item := range []struct {
		path, label string
		args        []string
	}{
		{serveAgentPath(), serveAgentLabel, []string{"gtmux", "serve", "--port", "9876"}},
		{tunnelAgentPath(), tunnelAgentLabel, []string{"/opt/cloudflared", "tunnel", "run", "--protocol", "http2", "--token", "old<&connector"}},
	} {
		if err := writeLaunchAgent(item.path, item.label, item.args, "/private-test-log"); err != nil {
			t.Fatal(err)
		}
	}
	writeTunnelURL("https://gtmux-oldlabel.example.dev")
	for _, file := range []string{"serve-token", "devices.json"} {
		if err := os.WriteFile(filepath.Join(state.ConfigDir(), file), []byte("existing-pairing-and-revocations\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := map[string][]byte{}
	for _, path := range []string{serveAgentPath(), tunnelAgentPath(), tunnelURLPath(), filepath.Join(state.ConfigDir(), "tunnel-device-id"), filepath.Join(state.ConfigDir(), "serve-token"), filepath.Join(state.ConfigDir(), "devices.json")} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		before[path] = b
	}
	return log, before
}

func assertRecoveryFiles(t *testing.T, before map[string][]byte, skipAgent bool) {
	t.Helper()
	for path, expected := range before {
		if skipAgent && path == tunnelAgentPath() {
			continue
		}
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("recovery changed %s", filepath.Base(path))
		}
	}
}

func TestAutomaticRecoveryRestoresPreviouslyDeletedDeviceWithoutRepairingPhonePairing(t *testing.T) {
	log, before := standardRecoveryFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request["deviceId"] != "previously-deleted-device-0001" || request["recover"] != true || request["force"] != nil {
			t.Error("old device identity was not retained")
		}
		_ = json.NewEncoder(w).Encode(provisionResp{URL: readTunnelURL(), Token: "replacement>&connector", Recovered: true, AddressPreserved: true})
	}))
	defer server.Close()
	if err := recoverStandard(context.Background(), server.URL, "gate"); err != nil {
		t.Fatal(err)
	}
	assertRecoveryFiles(t, before, true)
	agent, err := os.ReadFile(tunnelAgentPath())
	if err != nil {
		t.Fatal(err)
	}
	token, err := connectorFromPlist(agent)
	if err != nil || token != "replacement>&connector" {
		t.Fatal("new connector token was not applied safely")
	}
	if !bytes.Contains(agent, []byte("/opt/cloudflared")) || !bytes.Contains(agent, []byte("http2")) {
		t.Fatal("recovery changed existing connector arguments")
	}
	info, err := os.Stat(tunnelAgentPath())
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatal("connector credentials lost private permissions")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "com.gtmux.tunnel.plist") != 2 || strings.Contains(string(calls), "com.gtmux.serve") {
		t.Fatalf("wrong services restarted: %s", calls)
	}
}

func TestAutomaticRecoveryDoesNotRestartAHealthyUnchangedConnector(t *testing.T) {
	log, before := standardRecoveryFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(provisionResp{URL: readTunnelURL(), Token: "old<&connector", Recovered: true, AddressPreserved: true})
	}))
	defer server.Close()
	if err := recoverStandard(context.Background(), server.URL, "gate"); err != nil {
		t.Fatal(err)
	}
	assertRecoveryFiles(t, before, false)
	if b, _ := os.ReadFile(log); len(b) != 0 {
		t.Fatal("transient outage caused a connector restart")
	}
}

func TestAutomaticRecoveryRefusesProviderFailureOrAddressRotation(t *testing.T) {
	for _, scenario := range []string{"unauthorized", "outage", "old-worker", "changed-address", "unacknowledged"} {
		t.Run(scenario, func(t *testing.T) {
			log, before := standardRecoveryFixture(t)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				if scenario == "unauthorized" {
					w.WriteHeader(401)
					return
				}
				if scenario == "outage" {
					w.WriteHeader(503)
					return
				}
				p := provisionResp{URL: readTunnelURL(), Token: "new", Recovered: true, AddressPreserved: true}
				if scenario == "old-worker" {
					p.AddressPreserved = false
				}
				if scenario == "changed-address" {
					p.URL = "https://gtmux-new.example.dev"
				}
				if scenario == "unacknowledged" {
					p.Recovered = false
				}
				_ = json.NewEncoder(w).Encode(p)
			}))
			defer server.Close()
			if err := recoverStandard(context.Background(), server.URL, "gate"); err == nil {
				t.Fatal("unsafe response was accepted")
			}
			assertRecoveryFiles(t, before, false)
			if b, _ := os.ReadFile(log); len(b) != 0 {
				t.Fatal("failed recovery modified launchd")
			}
		})
	}
}

func TestAutomaticRecoveryHonorsOffLANAndDirectDuringNetworkRequest(t *testing.T) {
	for _, mode := range []string{"off", "lan", "direct", "identity-reset"} {
		t.Run(mode, func(t *testing.T) {
			_, _ = standardRecoveryFixture(t)
			entered, release := make(chan struct{}), make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				close(entered)
				<-release
				_ = json.NewEncoder(w).Encode(provisionResp{URL: "https://gtmux-oldlabel.example.dev", Token: "new", Recovered: true, AddressPreserved: true})
			}))
			defer server.Close()
			done := make(chan error, 1)
			go func() { done <- recoverStandard(context.Background(), server.URL, "gate") }()
			select {
			case <-entered:
			case <-time.After(5 * time.Second):
				t.Fatal("no provision request")
			}
			switch mode {
			case "off":
				if serviceRemoveAll() != 0 {
					t.Fatal("Off failed")
				}
			case "lan":
				if serveServiceInstall(9876) != 0 {
					t.Fatal("LAN switch failed")
				}
			case "direct":
				t.Setenv("GTMUX_SELFTUNNEL_URL", "https://direct.example.dev")
				t.Setenv("GTMUX_SELFTUNNEL_SECRET", "test-direct-secret")
				if tunnelSelfServiceInstall(9876, "Fixture", true) != 0 {
					t.Fatal("Direct switch failed")
				}

			case "identity-reset":
				_ = os.Remove(filepath.Join(state.ConfigDir(), "tunnel-device-id"))
			}
			close(release)
			select {
			case err := <-done:
				if err == nil {
					t.Fatal("stale recovery applied")
				}
			case <-time.After(5 * time.Second):
				t.Fatal("recovery did not finish")
			}
			if mode != "identity-reset" && fileExists(tunnelAgentPath()) {
				t.Fatal("Standard was re-enabled after a newer user choice")
			}
			if mode == "identity-reset" && fileExists(filepath.Join(state.ConfigDir(), "tunnel-device-id")) {
				t.Fatal("identity was recreated")
			}
		})
	}
}

func TestAutomaticRecoveryRequiresExistingOptInAndIdentity(t *testing.T) {
	for _, missing := range []string{"tunnel", "serve", "device", "credentials"} {
		t.Run(missing, func(t *testing.T) {
			_, _ = standardRecoveryFixture(t)
			path := map[string]string{"tunnel": tunnelAgentPath(), "serve": serveAgentPath(), "device": filepath.Join(state.ConfigDir(), "tunnel-device-id"), "credentials": filepath.Join(state.ConfigDir(), "serve-token")}[missing]
			_ = os.Remove(path)
			if err := recoverStandard(context.Background(), "http://127.0.0.1:1", "gate"); err == nil {
				t.Fatal("reset state accepted")
			}
			if fileExists(path) {
				t.Fatal("missing setting was recreated")
			}
		})
	}
}

func TestStandardRecoveryGraceSingleFlightAndBackoff(t *testing.T) {
	w := &standardRecoveryWatch{}
	now := time.Unix(1000, 0)
	if w.reserve(now, false) || w.reserve(now.Add(59*time.Second), false) {
		t.Fatal("recovery skipped connection grace")
	}
	if !w.reserve(now.Add(time.Minute), false) || w.reserve(now.Add(2*time.Hour), false) {
		t.Fatal("single-flight failed")
	}
	w.complete()
	if w.reserve(now.Add(90*time.Second), false) {
		t.Fatal("recovery ignored backoff")
	}
	for i := 0; i < 12; i++ {
		now = now.Add(2 * time.Hour)
		if !w.reserve(now, false) {
			t.Fatal("retry never resumed")
		}
		w.complete()
	}
	if w.delay != time.Hour {
		t.Fatal("backoff is not bounded")
	}
	if w.reserve(now, true) || !w.downSince.IsZero() || w.delay != 0 {
		t.Fatal("successful edge connection did not reset retries")
	}
}

func TestAutomaticRecoveryRetriesInterruptedLocalReload(t *testing.T) {
	log, before := standardRecoveryFixture(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(provisionResp{URL: readTunnelURL(), Token: "replacement-connector", Recovered: true, AddressPreserved: true})
	}))
	defer server.Close()
	bin := filepath.Dir(log)
	failure := filepath.Join(bin, "fail-load")
	if err := os.WriteFile(failure, []byte("fail"), 0600); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\n[ \"$1\" = list ] && exit 0\nprintf '%s %s\\n' \"$1\" \"$3\" >> \"" + log + "\"\n[ \"$1\" = load ] && [ -f \"" + failure + "\" ] && exit 1\nexit 0\n"
	if err := os.WriteFile(filepath.Join(bin, "launchctl"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	if err := recoverStandard(context.Background(), server.URL, "gate"); err == nil {
		t.Fatal("failed reload was reported as success")
	}
	marker := filepath.Join(state.ConfigDir(), "standard-recovery-pending")
	if !fileExists(marker) {
		t.Fatal("interrupted local application has no durable retry marker")
	}
	if err := os.Remove(failure); err != nil {
		t.Fatal(err)
	}
	if err := recoverStandard(context.Background(), server.URL, "gate"); err != nil {
		t.Fatal(err)
	}
	if fileExists(marker) {
		t.Fatal("completed application left the retry marker")
	}
	calls, _ := os.ReadFile(log)
	if strings.Count(string(calls), "com.gtmux.tunnel.plist") != 4 {
		t.Fatal("unchanged saved token hid an interrupted connector reload")
	}
	assertRecoveryFiles(t, before, true)
}

func TestAutomaticRecoveryCannotInvokeLegacyAddressRotatingProvisioner(t *testing.T) {
	_, before := standardRecoveryFixture(t)
	mutations := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/provision" {
			mutations++
			_ = json.NewEncoder(w).Encode(provisionResp{URL: "https://rotated.example", Token: "new", Recovered: true})
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	if err := recoverStandard(context.Background(), server.URL, "gate"); err == nil {
		t.Fatal("old server accepted stable recovery")
	}
	if mutations != 0 {
		t.Fatal("watcher invoked an old server's destructive recovery path")
	}
	assertRecoveryFiles(t, before, false)
}
