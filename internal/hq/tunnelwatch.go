package hq

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/hqsurface"
	"github.com/chenchaoyi/gtmux/internal/hqwake"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// tunnelWatch tells HQ when remote access goes down and when it comes back (openspec
// change `diagnostics`, task 2.9). The tunnel reports its own state in status/tunnel.json
// for both backends; this reads it, never a log, and speaks only on a transition.
//
// A down tunnel means the commander's phone cannot reach this Mac. HQ cannot repair that,
// but it is the one relaying to someone who may be away from the Mac, and "your phone is
// cut off" is news they should get before they find out by trying. The tunnel already
// damps flapping: it reads down only after a failure once connected, or a minute of
// failing to connect.
//
// A status that is missing or stale says nothing either way (the tunnel was turned off,
// or its writer died), so it changes nothing here; the first healthy sight of a tunnel
// is not news either.
func tunnelWatch(now int64) {
	st, fresh := diag.ReadStatus("tunnel")
	if !fresh {
		return
	}
	var key string
	switch st.State {
	case "down":
		key = "down"
	case "connected":
		key = "up"
	default:
		return // connecting: not yet a verdict
	}
	prev := state.ReadMarker(filepath.Join(state.Dir(), tunnelMarker))
	if !markerChanged(tunnelMarker, key) || (prev == "" && key == "up") {
		return
	}
	line, summary := tunnelNews(st, key, time.Unix(now, 0))
	events.Append(events.Record{Ts: now, Event: hqsurface.ControlTunnel, Summary: summary, Severity: events.SevNotable})
	nudgeHQPane(line, "")
}

const tunnelMarker = "tunnel-watch"

// tunnelNews builds the wake line and the journal summary for a transition. The error is
// the tunnel's own words about why it failed (a resolver error, an HTTP status), which is
// gtmux's text, not anyone's content.
func tunnelNews(st diag.Status, key string, now time.Time) (line, summary string) {
	backend, _ := st.Detail["backend"].(string)
	if backend == "" {
		backend = "?"
	}
	since := ""
	if !st.Since.IsZero() {
		since = now.Sub(st.Since).Round(time.Second).String()
	}
	if key == "down" {
		errText, _ := st.Detail["lastError"].(string)
		fields := []string{"backend: " + backend, "the phone cannot reach this Mac"}
		if errText != "" {
			// The tunnel's own words, quoted as data like every err: field.
			fields = append(fields, `err:"`+strings.ReplaceAll(errText, `"`, `'`)+`"`)
		}
		return hqwake.Line(hqwake.ClassTunnel, "down", fields...),
			strings.TrimSpace(fmt.Sprintf("tunnel down (%s): %s", backend, errText))
	}
	summary = "tunnel back up (" + backend + ")"
	if since != "" {
		summary += ", connected " + since + " ago"
	}
	return hqwake.Line(hqwake.ClassTunnel, "up", "backend: "+backend, "the phone can reach this Mac again"), summary
}
