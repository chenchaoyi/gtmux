package app

import (
	"errors"
	"strings"
	"testing"
)

// TestFriendlyTunnelError: raw provision errors become calm messages that never
// leak the internal service URL, "provision", or the raw Go error string.
func TestFriendlyTunnelError(t *testing.T) {
	cases := []struct {
		raw      string
		wantSubs string // an English substring the friendly message should contain
	}{
		{`Post "https://gtmux-tunnel.ccy-chenchaoyi.workers.dev/provision": EOF`, "check your internet"},
		{"dial tcp: i/o timeout", "check your internet"},
		{"HTTP 403: forbidden", "turned down"},
		{"incomplete provision response", "try again"},
	}
	for _, c := range cases {
		en, zh := friendlyTunnelError(errors.New(c.raw))
		if !strings.Contains(en, c.wantSubs) {
			t.Errorf("friendly(%q) en = %q, want substring %q", c.raw, en, c.wantSubs)
		}
		for _, leak := range []string{"workers.dev", "provision", "EOF", "Post \"", "http://", "https://"} {
			if strings.Contains(en, leak) || strings.Contains(zh, leak) {
				t.Errorf("friendly(%q) leaked internal detail %q: en=%q zh=%q", c.raw, leak, en, zh)
			}
		}
		if zh == "" {
			t.Errorf("friendly(%q) zh is empty", c.raw)
		}
	}
}
