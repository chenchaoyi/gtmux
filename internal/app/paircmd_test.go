package app

import (
	"strings"
	"testing"
)

// pairMedia renders the browser link and the terminal one-liner from one code.
func TestPairMedia(t *testing.T) {
	browser, attach := pairMedia("https://gtmux-abc.ccy.dev", "deadbeef")
	if browser != "https://gtmux-abc.ccy.dev/#c=deadbeef" {
		t.Fatalf("browser = %q", browser)
	}
	if attach != "gtmux attach 'https://gtmux-abc.ccy.dev/#c=deadbeef'" {
		t.Fatalf("attach = %q", attach)
	}
	// The attach one-liner must be single-quoted: the fragment would otherwise be
	// eaten by an interactive shell's comment/expansion handling.
	if !strings.Contains(attach, "'") {
		t.Fatal("attach one-liner must quote the URL")
	}
}
