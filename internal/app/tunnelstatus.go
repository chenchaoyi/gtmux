package app

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

// status/tunnel.json says whether this Mac is reachable from outside, for either
// backend (openspec change `diagnostics`, design 7.2). The pairing window and doctor
// read it. Before it, the window decided "the tunnel is down" by matching phrases in
// cloudflared's log for either backend, so a Direct user's verdict came from a tunnel
// they were not running, and the Direct client published nothing at all.

// Tunnel states.
const (
	tunnelConnecting = "connecting"
	tunnelConnected  = "connected"
	tunnelDown       = "down"
)

// tunnelStatusStale: a reporter rewrites at least every 30s, so 90s without a write
// means it has stopped.
const tunnelStatusStale = 90 * time.Second

// tunnelReporter publishes one backend's state and logs only its transitions, so a
// tunnel that stays down does not write a line every probe.
//
// States: connecting until the first success; connected after a success; down after a
// failure once connected, or when a minute of connecting has not succeeded.
type tunnelReporter struct {
	mu        sync.Mutex
	backend   string
	server    string
	pairURL   string
	started   time.Time
	state     string
	lastErr   string
	lastErrAt time.Time
	lastOK    time.Time
}

// connectGrace is how long a tunnel may keep failing before it counts as down rather
// than still dialling.
const connectGrace = time.Minute

func newTunnelReporter(backend, server, pairURL string) *tunnelReporter {
	return &tunnelReporter{backend: backend, server: server, pairURL: pairURL,
		started: time.Now(), state: tunnelConnecting}
}

// report records one probe: ok, or failed with err. report(false, "") only publishes.
func (t *tunnelReporter) report(ok bool, err string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	lg := diag.For("tunnel")
	switch {
	case ok:
		if t.state != tunnelConnected {
			lg.Info("tunnel.connected", "the tunnel reaches this Mac end to end",
				"backend", t.backend, "server", t.server)
		}
		t.state, t.lastOK = tunnelConnected, now
	case err != "":
		t.lastErr, t.lastErrAt = err, now
		lg.Debug("tunnel.probe.failed", "a tunnel probe failed", "backend", t.backend, "error", err)
		goesDown := t.state == tunnelConnected ||
			(t.state == tunnelConnecting && now.Sub(t.started) > connectGrace)
		if goesDown {
			t.state = tunnelDown
			lg.Warn("tunnel.disconnected", "the tunnel does not reach this Mac",
				"backend", t.backend, "server", t.server, "error", err)
		}
	}
	detail := map[string]any{"backend": t.backend, "server": t.server, "url": t.pairURL}
	if t.lastErr != "" {
		detail["lastError"] = t.lastErr
		detail["lastErrorAt"] = t.lastErrAt.Format(time.RFC3339)
	}
	if !t.lastOK.IsZero() {
		detail["lastOkAt"] = t.lastOK.Format(time.RFC3339)
	}
	diag.Publish("tunnel", t.state, tunnelStatusStale, detail)
}

// probeHealth asks a pairing URL's /api/health directly (no proxy: the tunnel client
// does not use one, and a proxy's path to the server says nothing about the tunnel).
func probeHealth(ctx context.Context, pairURL string) error {
	tr := &http.Transport{Proxy: nil}
	hc := &http.Client{Timeout: 8 * time.Second, Transport: tr}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(pairURL, "/")+"/api/health", nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("the pairing address answered HTTP %d", resp.StatusCode)
	}
	return nil
}

// watchSelfTunnel probes the Direct pairing URL every 30s (every 5s until the first
// success) for as long as the client runs, and reports each result. The probe resolves
// the name the client dials, so a network that hijacks it shows as down with the
// resolver's error, which is the truth for Direct.
func watchSelfTunnel(ctx context.Context, server string) *tunnelReporter {
	host := server
	if u, err := url.Parse(server); err == nil && u.Host != "" {
		host = u.Host
	}
	rep := newTunnelReporter("direct", host, selfTunnelPairURL(server))
	rep.report(false, "")
	go func() {
		for {
			err := probeHealth(ctx, rep.pairURL)
			if ctx.Err() != nil {
				return
			}
			if err == nil {
				rep.report(true, "")
			} else {
				rep.report(false, err.Error())
			}
			wait := 30 * time.Second
			rep.mu.Lock()
			if rep.lastOK.IsZero() {
				wait = 5 * time.Second
			}
			rep.mu.Unlock()
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return
			}
		}
	}()
	return rep
}

// cloudflaredMetricsAddr is where gtmux asks cloudflared to publish its metrics, so the
// Standard tunnel's state is read from its registered-connection count rather than from
// phrases in its log. The ports after it are cloudflared's own defaults, for a service
// installed before gtmux passed the address explicitly.
const cloudflaredMetricsAddr = "127.0.0.1:49317"

var cloudflaredMetricsFallbacks = []string{"127.0.0.1:20241", "127.0.0.1:20242", "127.0.0.1:20243", "127.0.0.1:20244", "127.0.0.1:20245"}

// haConnections reads cloudflared_tunnel_ha_connections from a Prometheus text page.
func haConnections(body string) (int, bool) {
	sc := bufio.NewScanner(strings.NewReader(body))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "cloudflared_tunnel_ha_connections ") {
			f := strings.Fields(line)
			if n, err := strconv.ParseFloat(f[len(f)-1], 64); err == nil {
				return int(n), true
			}
		}
	}
	return 0, false
}

var standardReporter *tunnelReporter

// sampleStandardTunnel is serve's slow-tick step under the Standard backend: read
// cloudflared's metrics and report. No-op for any other backend.
func sampleStandardTunnel() {
	if tunnelBackend() != "standard" {
		return
	}
	if standardReporter == nil {
		standardReporter = newTunnelReporter("standard", "cloudflare", readTunnelURL())
	}
	hc := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}}
	for _, addr := range append([]string{cloudflaredMetricsAddr}, cloudflaredMetricsFallbacks...) {
		resp, err := hc.Get("http://" + addr + "/metrics")
		if err != nil {
			continue
		}
		b := new(strings.Builder)
		_, _ = bufio.NewReader(resp.Body).WriteTo(b)
		_ = resp.Body.Close()
		if n, ok := haConnections(b.String()); ok {
			if n > 0 {
				standardReporter.report(true, "")
			} else {
				standardReporter.report(false, "cloudflared has no registered connection to the edge")
			}
			return
		}
	}
	standardReporter.report(false, "cloudflared's metrics are not reachable: is the tunnel service running?")
}
