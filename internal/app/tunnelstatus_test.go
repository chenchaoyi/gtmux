package app

import (
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
