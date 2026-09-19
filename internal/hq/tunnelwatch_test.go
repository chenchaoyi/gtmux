package hq

import (
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqsurface"
)

// HQ hears about remote access on a transition only: a healthy tunnel first seen is not
// news, going down is, staying down is not again, and coming back is.
func TestHQHearsWhenTheTunnelGoesDownAndComesBack(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Now().Unix()
	records := func() []string {
		var out []string
		for _, r := range events.Read(0, now+10) {
			if r.Event == hqsurface.ControlTunnel {
				out = append(out, r.Summary)
			}
		}
		return out
	}
	publish := func(state, lastError string) {
		diag.Publish("tunnel", state, 90*time.Second, map[string]any{"backend": "direct", "lastError": lastError})
	}

	publish("connected", "")
	tunnelWatch(now)
	if got := records(); len(got) != 0 {
		t.Fatalf("a healthy tunnel seen first is not news: %v", got)
	}

	publish("down", "lookup tunnel.example.dev: no such host")
	tunnelWatch(now)
	tunnelWatch(now) // still down: said once
	got := records()
	if len(got) != 1 || !strings.Contains(got[0], "tunnel down (direct)") || !strings.Contains(got[0], "no such host") {
		t.Fatalf("after going down: %v", got)
	}

	publish("connecting", "")
	tunnelWatch(now)
	publish("connected", "")
	tunnelWatch(now)
	got = records()
	if len(got) != 2 || !strings.Contains(got[1], "tunnel back up (direct)") {
		t.Fatalf("after coming back: %v", got)
	}

	// A status whose writer is gone says nothing, even one that reads down: a stale
	// staleAfter of 0 is never fresh.
	diag.Publish("tunnel", "down", 0, map[string]any{"backend": "direct"})
	tunnelWatch(now)
	if n := len(records()); n != 2 {
		t.Fatalf("a stale status produced a record: %d", n)
	}
}

func TestTheTunnelLineIsATunnelWake(t *testing.T) {
	line, _ := tunnelNews(diag.Status{Detail: map[string]any{"backend": "standard", "lastError": `bad "gateway"`}}, "down", time.Now())
	if !strings.Contains(line, "gtmux·tunnel") || !strings.Contains(line, `err:"bad 'gateway'"`) {
		t.Fatalf("line %q", line)
	}
}
