package app

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/i18n"
)

// Every path that prints a pairing block (foreground, --service, a reused service,
// Direct) says what the pairing hands out: control of this Mac, terminal input
// included. It used to call the browser a "view-only mirror", and only the stable
// foreground footer carried a warning, which said "read your radar" (%12's review of
// #1387, 2026-10-06).
func TestPairingBlockSaysItGrantsControl(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := i18n.Lang()
	i18n.SetLang("en")
	t.Cleanup(func() { i18n.SetLang(old) }) // SetLang("") would be a no-op

	// A serve that mints a code: the block offers the one-time code and the browser link.
	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/enroll/mint" {
			_, _ = w.Write([]byte(`{"enrollCode":"c0ffee"}`))
		}
	}))
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	srv.Listener = ln
	srv.Start()
	defer srv.Close()
	port, _ := strconv.Atoi(strings.TrimPrefix(ln.Addr().String()[strings.LastIndex(ln.Addr().String(), ":"):], ":"))

	minted := captureStdout(t, func() { printPairingBlock("https://gtmux-x.example", "master-token", "mac", port) })
	for _, want := range []string{"can read and control this Mac", "terminal input included", "any device paired from this code", "/#c=c0ffee", "it can watch and type"} {
		if !strings.Contains(minted, want) {
			t.Errorf("minted pairing block lacks %q:\n%s", want, minted)
		}
	}
	// No serve to mint from: the QR falls back to the token itself, and says so.
	legacy := captureStdout(t, func() { printPairingBlock("https://gtmux-x.example", "master-token", "mac", 0) })
	for _, want := range []string{"carries the owner token itself", "can read and control this Mac"} {
		if !strings.Contains(legacy, want) {
			t.Errorf("token pairing block lacks %q:\n%s", want, legacy)
		}
	}
	for _, out := range []string{minted, legacy} {
		for _, gone := range []string{"view-only", "read your radar", "from anywhere"} {
			if strings.Contains(out, gone) {
				t.Errorf("pairing block still says %q:\n%s", gone, out)
			}
		}
	}
}
