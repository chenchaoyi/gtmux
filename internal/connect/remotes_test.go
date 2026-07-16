package connect

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Save/Load round-trip, key normalization, and the 0600 mode (credentials file).
func TestRemotesStore(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if got := LoadRemoteToken("http://h:1"); got != "" {
		t.Fatalf("empty store must yield \"\", got %q", got)
	}
	if err := SaveRemoteToken("http://h:1/", "tok-a"); err != nil {
		t.Fatal(err)
	}
	if err := SaveRemoteToken("http://h2:2", "tok-b"); err != nil {
		t.Fatal(err)
	}
	if got := LoadRemoteToken("http://h:1"); got != "tok-a" { // trailing-slash normalized
		t.Fatalf("load = %q, want tok-a", got)
	}
	if got := LoadRemoteToken("http://h2:2"); got != "tok-b" {
		t.Fatalf("load = %q, want tok-b", got)
	}
	fi, err := os.Stat(remotesPath())
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("remotes.json mode = %v, want 0600 (it holds live credentials)", fi.Mode().Perm())
	}
}

// RedeemEnrollCode exchanges the code for a token via POST /api/enroll.
func TestRedeemEnrollCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/enroll" || r.Method != http.MethodPost {
			http.Error(w, "wrong route", 404)
			return
		}
		var body struct{ EnrollCode, Name string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body.EnrollCode != "code-1" {
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or expired enroll code"})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"token": "minted-tok", "deviceId": "d1"})
	}))
	defer srv.Close()

	tok, err := RedeemEnrollCode(context.Background(), srv.URL, "code-1", "my-laptop")
	if err != nil || tok != "minted-tok" {
		t.Fatalf("redeem = (%q, %v)", tok, err)
	}
	if _, err := RedeemEnrollCode(context.Background(), srv.URL, "expired", "x"); err == nil {
		t.Fatal("an expired code must error")
	}
}
