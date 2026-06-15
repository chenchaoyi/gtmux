package menubar

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBadgeText(t *testing.T) {
	cases := []struct {
		name   string
		agents []Agent
		want   string
	}{
		{"empty", nil, ""},
		{"all idle", []Agent{{Status: "idle"}, {Status: "running"}}, ""},
		{"working count", []Agent{{Status: "working"}, {Status: "working"}}, "2"},
		{"waiting wins", []Agent{{Status: "working"}, {Status: "waiting"}}, "1"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := BadgeText(c.agents); got != c.want {
				t.Errorf("BadgeText = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFilterWaiting(t *testing.T) {
	agents := []Agent{
		{PaneID: "%1", Status: "waiting"},
		{PaneID: "%2", Status: "working"},
		{PaneID: "%3", Status: "waiting"},
		{PaneID: "%4", Status: "idle"},
	}
	got := FilterWaiting(agents)
	if len(got) != 2 || got[0].PaneID != "%1" || got[1].PaneID != "%3" {
		t.Errorf("FilterWaiting = %+v, want the two waiting agents", got)
	}
}

func TestIconColor(t *testing.T) {
	cases := []struct {
		agents []Agent
		want   string // color name for readable failure messages
	}{
		{nil, "none"},
		{[]Agent{{Status: "idle"}}, "idle"},
		{[]Agent{{Status: "working"}, {Status: "idle"}}, "working"},
		{[]Agent{{Status: "waiting"}, {Status: "working"}}, "waiting"},
	}
	names := map[any]string{colorNone: "none", colorIdle: "idle", colorWorking: "working", colorWaiting: "waiting"}
	for _, c := range cases {
		if got := names[IconColor(c.agents)]; got != c.want {
			t.Errorf("IconColor(%v) = %s, want %s", c.agents, got, c.want)
		}
	}
}

func TestIconForIsValidPNG(t *testing.T) {
	b := IconFor([]Agent{{Status: "waiting"}})
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("IconFor did not produce a decodable PNG: %v", err)
	}
	// Center pixel should be the (opaque) waiting color.
	bnd := img.Bounds()
	r, g, bl, a := img.At(bnd.Dx()/2, bnd.Dy()/2).RGBA()
	if a>>8 != 0xFF || r>>8 != 0xEF || g>>8 != 0x44 || bl>>8 != 0x44 {
		t.Errorf("center pixel = (%d,%d,%d,%d), want opaque waiting-red", r>>8, g>>8, bl>>8, a>>8)
	}
	// Corner should be transparent (outside the dot).
	if _, _, _, ca := img.At(0, 0).RGBA(); ca != 0 {
		t.Errorf("corner alpha = %d, want 0 (transparent)", ca>>8)
	}
}

func TestSqrt(t *testing.T) {
	for _, x := range []float64{0, 1, 4, 9, 2, 100} {
		got := sqrt(x)
		if d := got*got - x; d > 1e-6 || d < -1e-6 {
			t.Errorf("sqrt(%v) = %v (squared off by %v)", x, got, d)
		}
	}
}

func TestWatchStateFiresOnChange(t *testing.T) {
	dir := t.TempDir()
	events, stop, err := WatchState(dir, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("WatchState: %v", err)
	}
	defer stop()

	// Touch a waiting marker; expect a (debounced) signal.
	if err := os.WriteFile(filepath.Join(dir, "waiting", "%5"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	select {
	case <-events:
	case <-time.After(3 * time.Second):
		t.Fatal("expected a watch event after creating waiting/%5, got none")
	}
}
