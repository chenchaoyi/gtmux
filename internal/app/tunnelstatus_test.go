package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHAConnectionsReadsCloudflaredsGauge(t *testing.T) {
	page := "# HELP cloudflared_tunnel_ha_connections Number of active ha connections\n" +
		"# TYPE cloudflared_tunnel_ha_connections gauge\n" +
		"cloudflared_tunnel_ha_connections 4\n" +
		"cloudflared_tunnel_total_requests 12\n"
	if n, ok := haConnections(page); !ok || n != 4 {
		t.Errorf("haConnections = %d, %v; want 4, true", n, ok)
	}
	if _, ok := haConnections("go_goroutines 9\n"); ok {
		t.Error("a page without the gauge was read as having one")
	}
}

// A handshake in progress is not a connection: cloudflared's gauge counts it, /ready
// does not. Behind a proxy that cut every handshake, the gauge read 2 for five seconds a
// minute and serve called the tunnel up each time (2026-10-05).
func TestStandardTunnelCountsRegisteredConnectionsNotDials(t *testing.T) {
	gauge := "cloudflared_tunnel_ha_connections 2\n"
	for _, tc := range []struct {
		name  string
		ready string // "" = no /ready (an older cloudflared answers 404 with no JSON)
		code  int
		want  int
	}{
		{"dialling, none registered", `{"status":503,"readyConnections":0,"connectorId":"x"}`, 503, 0},
		{"registered", `{"status":200,"readyConnections":2,"connectorId":"x"}`, 200, 2},
		{"cloudflared without /ready", "", 404, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/ready":
					w.WriteHeader(tc.code)
					_, _ = io.WriteString(w, tc.ready)
				case "/metrics":
					_, _ = io.WriteString(w, gauge)
				}
			}))
			defer srv.Close()
			n, ok := cloudflaredConnections(srv.Client(), strings.TrimPrefix(srv.URL, "http://"))
			if !ok || n != tc.want {
				t.Fatalf("cloudflaredConnections = %d, %v; want %d", n, ok, tc.want)
			}
		})
	}
	if _, ok := cloudflaredConnections(&http.Client{}, "127.0.0.1:1"); ok {
		t.Error("an address that answers nothing was read as a count")
	}
}

// The states a reader acts on, and a log that records transitions, not every probe.
func TestTheTunnelReporterRecordsTransitionsNotEveryProbe(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	rep := newTunnelReporter("direct", "tunnel.example.com", "https://tunnel.example.com/p30000")

	rep.report(false, "dial tcp: lookup tunnel.example.com: no such host")
	if st, fresh := diag.ReadStatus("tunnel"); !fresh || st.State != tunnelConnecting {
		t.Fatalf("a first failure should read as still connecting, got %q fresh=%v", st.State, fresh)
	}
	rep.started = time.Now().Add(-2 * time.Minute) // past the grace
	for i := 0; i < 5; i++ {
		rep.report(false, "dial tcp: lookup tunnel.example.com: no such host")
	}
	st, _ := diag.ReadStatus("tunnel")
	if st.State != tunnelDown || !strings.Contains(st.Detail["lastError"].(string), "no such host") {
		t.Errorf("after a minute of failing: state %q, detail %v", st.State, st.Detail)
	}
	rep.report(true, "")
	rep.report(true, "")
	if st, _ := diag.ReadStatus("tunnel"); st.State != tunnelConnected || st.Detail["backend"] != "direct" {
		t.Errorf("after a success: %+v", st)
	}

	b, _ := os.ReadFile(filepath.Join(state.LogsDir(), time.Now().Format("2006-01-02")+".jsonl"))
	log := string(b)
	if n := strings.Count(log, `"event":"tunnel.disconnected"`); n != 1 {
		t.Errorf("tunnel.disconnected written %d times for one outage, want 1", n)
	}
	if n := strings.Count(log, `"event":"tunnel.connected"`); n != 1 {
		t.Errorf("tunnel.connected written %d times for one recovery, want 1", n)
	}
}
