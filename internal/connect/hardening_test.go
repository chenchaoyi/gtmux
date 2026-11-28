package connect

import (
	"strings"
	"testing"
)

func TestCHH006_AttachWSURLCarriesExplicitPredictFlag(t *testing.T) {
	t.Setenv("TERM", "xterm-256color")
	off := attachWSURL("https://example.com/", "%3", false)
	if !strings.HasPrefix(off, "wss://example.com/api/attach?") || !strings.HasSuffix(off, "&predict=0") {
		t.Fatalf("attachWSURL(..., false) = %q, want wss URL ending with &predict=0", off)
	}
	on := attachWSURL("http://127.0.0.1:8765", "%3", true)
	if !strings.HasPrefix(on, "ws://127.0.0.1:8765/api/attach?") || !strings.HasSuffix(on, "&predict=1") {
		t.Fatalf("attachWSURL(..., true) = %q, want ws URL ending with &predict=1", on)
	}
}
