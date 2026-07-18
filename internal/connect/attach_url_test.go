package connect

import "testing"

func TestWSURL(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	cases := []struct{ name, base, pane, want string }{
		{"http→ws", "http://host:8765", "%1", "ws://host:8765/api/attach?id=%251&term=xterm-256color"},
		{"https→wss", "https://x.ccy.dev", "%1", "wss://x.ccy.dev/api/attach?id=%251&term=xterm-256color"},
		{"trailing slash trimmed", "http://host/", "%2", "ws://host/api/attach?id=%252&term=xterm-256color"},
		{"pane id percent-escaped", "http://h", "%83", "ws://h/api/attach?id=%2583&term=xterm-256color"},
		{"non-http base left as-is (scheme untouched)", "host:8765", "%1", "host:8765/api/attach?id=%251&term=xterm-256color"},
	}
	for _, c := range cases {
		if got := wsURL(c.base, c.pane); got != c.want {
			t.Errorf("%s: wsURL(%q,%q) = %q, want %q", c.name, c.base, c.pane, got, c.want)
		}
	}
}

func TestWSURL_EmptyTERM(t *testing.T) {
	t.Setenv("TERM", "")
	if got := wsURL("http://h", "%1"); got != "ws://h/api/attach?id=%251&term=" {
		t.Errorf("empty TERM: %q", got)
	}
}
