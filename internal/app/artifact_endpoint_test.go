package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

type previewWorkspace struct {
	port   int
	boot   string
	server *httptest.Server
}

func newPreviewWorkspace(t *testing.T) previewWorkspace {
	t.Helper()
	const boot = "0123456789abcdef"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/health" || r.Method != "GET" || r.Header.Get("Authorization") != "" {
			t.Error("unexpected preview probe")
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "service": "gtmux", "boot": boot})
	}))
	t.Cleanup(server.Close)
	u, _ := url.Parse(server.URL)
	port, _ := strconv.Atoi(u.Port())
	workspace := previewWorkspace{port, boot, server}
	workspace.publish(boot, time.Minute)
	return workspace
}

func (w previewWorkspace) publish(boot string, ttl time.Duration) {
	diag.Publish("serve", "running", ttl, map[string]any{"port": w.port, "bind": "127.0.0.1", "boot": boot})
}

func TestLAP007OnlyCurrentWorkspaceOffersALocalPreview(t *testing.T) {
	file := localPreviewFixture(t)
	var receipt struct {
		Reference string `json:"reference"`
		URL       string `json:"local_preview_url"`
	}
	out := captureStdout(t, func() {
		if cmdArtifact([]string{"add", file.Source, "--session-id", file.SessionID, "--home-id", file.HomeID, "--json"}) != 0 {
			t.Fatal("offline publication failed")
		}
	})
	if err := json.Unmarshal([]byte(out), &receipt); err != nil || receipt.Reference != file.Reference() || receipt.URL != "" {
		t.Fatal(out, err)
	}
	if cmdArtifact([]string{"open", file.ID, "--print-url"}) != 1 {
		t.Fatal("offline preview accepted")
	}
	w := newPreviewWorkspace(t)
	address, err := localArtifactPreviewURL(file, 0)
	if err != nil || !strings.HasPrefix(address, w.server.URL+"/#") {
		t.Fatal(address, err)
	}
	if explicit, err := localArtifactPreviewURL(file, w.port); err != nil || explicit != address {
		t.Fatal(explicit, err)
	}
	if _, err := localArtifactPreviewURL(file, 1+w.port%65535); err == nil {
		t.Fatal("wrong explicit port accepted")
	}
	w.publish("bbbbbbbbbbbbbbbb", time.Minute)
	if _, err := localArtifactPreviewURL(file, 0); err == nil {
		t.Fatal("different boot accepted")
	}
	w.publish(w.boot, -time.Second)
	if _, err := localArtifactPreviewURL(file, 0); err == nil {
		t.Fatal("stale status accepted")
	}
	w.publish(w.boot, time.Minute)
	w.server.Close()
	if _, err := localArtifactPreviewURL(file, 0); err == nil {
		t.Fatal("closed listener accepted")
	}
}

func TestLAP007LoopbackAddressesUseRecordedPort(t *testing.T) {
	for bind, want := range map[string]string{"0.0.0.0": "http://127.0.0.1:8765", "127.0.0.1": "http://127.0.0.1:8765", "::": "http://[::1]:8765", "::1": "http://[::1]:8765", "example.test": ""} {
		got, _, err := previewEndpoint(map[string]any{"port": float64(defaultServePort), "bind": bind, "boot": "0123456789abcdef"}, 0)
		if got != want || (err == nil) != (want != "") {
			t.Fatal(bind, got, err)
		}
	}
}

func TestLAP007UnexpectedHealthNeverProvidesAnAddress(t *testing.T) {
	file := localPreviewFixture(t)
	for _, reply := range []string{"incomplete", "oversized", "redirect"} {
		t.Run(reply, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/health" || r.Header.Get("Authorization") != "" {
					t.Error("unexpected request")
				}
				switch reply {
				case "incomplete":
					_, _ = w.Write([]byte(`{"status":"ok"}`))
				case "oversized":
					_, _ = w.Write([]byte(strings.Repeat(" ", 4097)))
				case "redirect":
					http.Redirect(w, r, "/other", http.StatusFound)
				}
			}))
			defer server.Close()
			address, _ := url.Parse(server.URL)
			port, _ := strconv.Atoi(address.Port())
			workspace := previewWorkspace{port: port}
			workspace.publish("0123456789abcdef", time.Minute)
			if got, err := localArtifactPreviewURL(file, port); err == nil || got != "" {
				t.Fatal("unexpected health response accepted", got, err)
			}
		})
	}
}
