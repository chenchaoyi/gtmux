package connect

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParsePaneChoice(t *testing.T) {
	const n = 3
	cases := []struct {
		in         string
		wantIdx    int
		wantCancel bool
		wantOK     bool
	}{
		{"", 0, false, true},     // Enter → default row 0
		{"\n", 0, false, true},   // bare newline → default
		{"1", 0, false, true},    // first
		{"3", 2, false, true},    // last
		{"  2 ", 1, false, true}, // whitespace tolerated
		{"q", 0, true, true},     // cancel
		{"Q", 0, true, true},     // cancel (upper)
		{"quit", 0, true, true},  // cancel (word)
		{"\x1b", 0, true, true},  // leading ESC → cancel
		{"0", 0, false, false},   // out of range (low)
		{"4", 0, false, false},   // out of range (high)
		{"x", 0, false, false},   // non-numeric
		{"1x", 0, false, false},  // trailing garbage
	}
	for _, c := range cases {
		idx, cancel, ok := parsePaneChoice(c.in, n)
		if idx != c.wantIdx || cancel != c.wantCancel || ok != c.wantOK {
			t.Errorf("parsePaneChoice(%q,%d) = (%d,%v,%v); want (%d,%v,%v)",
				c.in, n, idx, cancel, ok, c.wantIdx, c.wantCancel, c.wantOK)
		}
	}
}

func TestFormatPaneChoice(t *testing.T) {
	cases := []struct {
		a    Agent
		want string
	}{
		{Agent{Session: "work", Agent: "claude", Status: "waiting", Task: "run tests"},
			"work · claude · waiting  run tests"},
		{Agent{Session: "work", Agent: "claude", Status: "idle"},
			"work · claude · idle"},
		{Agent{PaneID: "%7"}, "%7"}, // no descriptive fields → pane id
		{Agent{Session: "s", Task: "x"}, "s  x"},
	}
	for _, c := range cases {
		if got := formatPaneChoice(c.a); got != c.want {
			t.Errorf("formatPaneChoice(%+v) = %q; want %q", c.a, got, c.want)
		}
	}
}

func TestFormatPaneChoiceTruncatesLongTask(t *testing.T) {
	long := ""
	for i := 0; i < 80; i++ {
		long += "x"
	}
	got := formatPaneChoice(Agent{Session: "s", Task: long})
	// 60-rune cap: 57 runes + ellipsis, prefixed by "s  ".
	if r := []rune(got); len(r) != len("s  ")+58 { // 57 + "…"
		t.Errorf("truncated length = %d runes; want %d", len(r), len("s  ")+58)
	}
	if got[len(got)-3:] != "…" {
		t.Errorf("want trailing ellipsis, got %q", got)
	}
}

// A share link gets no terminal (#1372): the serve refuses it, and the CLI says so before
// it spends anything. A share CODE is one-time, so redeeming it only to be refused would
// burn it; and keeping the link's token would offer an attach that can never work.
func shareServer(t *testing.T, all bool) (*httptest.Server, map[string]int) {
	t.Helper()
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		switch r.URL.Path {
		case "/api/enroll":
			_, _ = w.Write([]byte(`{"token":"guest-token","deviceId":"d1"}`))
		case "/api/health":
			_, _ = w.Write([]byte(`{"ok":true}`))
		case "/api/share":
			if all {
				_, _ = w.Write([]byte(`{"enabled":false,"panes":[],"all":true}`))
			} else {
				_, _ = w.Write([]byte(`{"enabled":false,"panes":[],"all":false}`))
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, calls
}

func TestAShareCodeIsRefusedBeforeItIsSpent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, calls := shareServer(t, false)
	if rc := Run([]string{srv.URL, "--code", "r97-k1v", "%1"}); rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if len(calls) != 0 {
		t.Errorf("a refused share code still reached the Mac: %v", calls)
	}
	if tok := LoadRemoteToken(srv.URL); tok != "" {
		t.Errorf("a token was kept: %q", tok)
	}
}

func TestAShareLinkIsRefusedAndNotKept(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, calls := shareServer(t, false)
	for _, link := range []string{
		srv.URL + "/#g=60f292edb483660acfab92cbd274fefac9e95b10c0a20d47416a935e41663ff2",
		srv.URL + "/#code=4F7K-Q9X2",
	} {
		if rc := Run([]string{link, "%1"}); rc != 1 {
			t.Errorf("%s: rc = %d, want 1", link, rc)
		}
	}
	if len(calls) != 0 {
		t.Errorf("a refused share link still reached the Mac: %v", calls)
	}
	if tok := LoadRemoteToken(srv.URL); tok != "" {
		t.Errorf("a token was kept: %q", tok)
	}
}

// A guest token reached another way (--token, or kept by an older gtmux) is told the same
// once the Mac says it is one, and no pane is asked for. No pane is given, so a client that
// went on would ask the Mac for its panes to pick one.
func TestAGuestTokenOnAHostIsRefused(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	srv, calls := shareServer(t, false)
	if rc := Run([]string{srv.URL, "--token", "guest-token"}); rc != 1 {
		t.Fatalf("rc = %d, want 1", rc)
	}
	if calls["/api/agents"] != 0 || calls["/api/attach"] != 0 {
		t.Errorf("a guest token went on to the panes: %v", calls)
	}
}
