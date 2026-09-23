package connect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// `gtmux attach <host>` has to keep working when that host moves to another Direct
// server: the address changes, the pairing does not (openspec/changes/direct-server-choice).

func TestARecordWrittenBeforeAddressesExistedStillAuthenticates(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// The old shape: base → token, a bare string.
	if err := os.MkdirAll(dirOf(remotesPath()), 0o755); err != nil {
		t.Fatal(err)
	}
	old, _ := json.Marshal(map[string]string{"https://sh.example.test/p35047": "tok-1"})
	if err := os.WriteFile(remotesPath(), old, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LoadRemoteToken("https://sh.example.test/p35047"); got != "tok-1" {
		t.Fatalf("token = %q, want the one this terminal already earned", got)
	}
	// And it gains addresses without losing the token.
	if err := SaveRemoteAddresses("https://sh.example.test/p35047", []string{"https://la.example.test/p35047"}); err != nil {
		t.Fatal(err)
	}
	r := LoadRemote("https://sh.example.test/p35047")
	if r.Token != "tok-1" || len(r.Alts) != 1 {
		t.Fatalf("remote = %+v", r)
	}
}

func TestAHostThatMovedIsFoundThroughTheAddressesItGave(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	moved := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health is unauthenticated on a real serve; everything else takes the token.
		if r.URL.Path == "/api/health" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok-1" {
			w.WriteHeader(401)
			return
		}
		if r.URL.Path == "/api/share" {
			_, _ = w.Write([]byte(`{"input":true,"all":true,"panes":[],"view_panes":[]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer moved.Close()
	dead := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	deadURL := dead.URL
	dead.Close()

	if err := SaveRemoteToken(deadURL, "tok-1"); err != nil {
		t.Fatal(err)
	}
	if err := SaveRemoteAddresses(deadURL, []string{deadURL, moved.URL}); err != nil {
		t.Fatal(err)
	}

	got := FindMoved(context.Background(), deadURL, "tok-1")
	if got != moved.URL {
		t.Fatalf("FindMoved = %q, want %q", got, moved.URL)
	}

	// The pairing moves with the host: the next bare attach to the new address is authed.
	if err := MoveRemote(deadURL, got); err != nil {
		t.Fatal(err)
	}
	if LoadRemoteToken(got) != "tok-1" {
		t.Fatal("the token did not move with the host")
	}
	if LoadRemoteToken(deadURL) != "" {
		t.Fatal("the old address was left looking live")
	}
}

func TestAnAddressThatAnswersToSomeoneElseIsNotThisHost(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	// Something is listening there, but it does not take our token: not our Mac, and the
	// terminal must not move its pairing onto it.
	stranger := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			_, _ = w.Write([]byte(`{"status":"ok"}`))
			return
		}
		w.WriteHeader(401)
	}))
	defer stranger.Close()

	if err := SaveRemoteToken("https://gone.example.test/p1", "tok-1"); err != nil {
		t.Fatal(err)
	}
	if err := SaveRemoteAddresses("https://gone.example.test/p1", []string{stranger.URL}); err != nil {
		t.Fatal(err)
	}
	if got := FindMoved(context.Background(), "https://gone.example.test/p1", "tok-1"); got != "" {
		t.Fatalf("FindMoved = %q, want nothing: that host did not take our token", got)
	}
}

func TestAHostThisTerminalNeverPairedWithRemembersNothing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := SaveRemoteAddresses("https://stranger.example.test/p1", []string{"https://x.example.test/p1"}); err != nil {
		t.Fatal(err)
	}
	if r := LoadRemote("https://stranger.example.test/p1"); r.Token != "" || len(r.Alts) != 0 {
		t.Fatalf("an unpaired host got a record: %+v", r)
	}
}

func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return p[:i]
		}
	}
	return "."
}
