package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Real loopback HTTP and the production handler/manager; no user serve, codes,
// tokens, disk roster, tmux or tunnel. Never log response credentials.
func TestRepeatPairingHTTP(t *testing.T) {
	en := NewEnrollManager(nil, nil)
	srv := httptest.NewServer(New(Config{Token: "synthetic-master"}, Deps{
		Enroll: en, AgentsJSON: func() ([]byte, error) { return []byte("[]"), nil },
	}).Handler())
	defer srv.Close()
	redeem := func(code string, read bool) (int, string) {
		t.Helper()
		payload, _ := json.Marshal(map[string]string{"enrollCode": code, "name": "Same iPad"})
		r, err := http.Post(srv.URL+"/api/enroll", "application/json", bytes.NewReader(payload))
		if err != nil {
			t.Error("synthetic enroll request failed")
			return 0, ""
		}
		defer r.Body.Close()
		if !read {
			return r.StatusCode, ""
		}
		var body struct {
			Token string `json:"token"`
		}
		if r.StatusCode == 200 {
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				t.Error("invalid response")
			}
		}
		return r.StatusCode, body.Token
	}
	code := en.Mint()
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); status, _ := redeem(code, true); statuses <- status }()
	}
	wg.Wait()
	close(statuses)
	ok, refused := 0, 0
	for status := range statuses {
		if status == 200 {
			ok++
		}
		if status == 401 {
			refused++
		}
	}
	if ok != 1 || refused != 1 || len(en.Devices()) != 1 {
		t.Fatal("one code must create exactly one device")
	}
	old := en.Devices()[0].Token
	// A reply lost after the server committed still spends the code. Retry must
	// refuse it; only a fresh code recovers. Neither failure nor fresh enrollment
	// revokes an already working credential.
	lost := en.Mint()
	if status, _ := redeem(lost, false); status != 200 {
		t.Fatal("first redemption failed")
	}
	if status, _ := redeem(lost, true); status != 401 {
		t.Fatal("spent code was accepted")
	}
	if status, fresh := redeem(en.Mint(), true); status != 200 || fresh == "" || fresh == old {
		t.Fatal("fresh code must issue a new credential")
	}
	if !en.ValidToken(old) {
		t.Fatal("previous connection was revoked")
	}
	// The same token on another installation is refused; no credential crossover.
	other := New(Config{Token: "synthetic-other"}, Deps{Enroll: NewEnrollManager(nil, nil), AgentsJSON: func() ([]byte, error) { return []byte("[]"), nil }}).Handler()
	req := httptest.NewRequest("GET", "/api/agents", nil)
	req.Header.Set("Authorization", "Bearer "+old)
	rr := httptest.NewRecorder()
	other.ServeHTTP(rr, req)
	if rr.Code != 401 {
		t.Fatal("another server accepted the credential")
	}
	req, _ = http.NewRequest("GET", srv.URL+"/api/agents", nil)
	req.Header.Set("Authorization", "Bearer "+old)
	r, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal("old connection request failed")
	}
	io.Copy(io.Discard, r.Body)
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatal("old connection stopped working")
	}
}

func TestRepeatPairingExpiryAndRestart(t *testing.T) {
	now := time.Unix(1000000, 0)
	en := NewEnrollManager(nil, nil)
	en.now = fixedClock(now)
	code := en.Mint()
	en.now = fixedClock(now.Add(5*time.Minute + time.Second))
	if _, why := en.RedeemWhy(code, "iPhone"); why != RedeemExpired {
		t.Fatal("five-minute code expiry changed")
	}
	code = en.Mint()
	restarted := NewEnrollManager(en.Devices(), nil)
	if _, why := restarted.RedeemWhy(code, "iPhone"); why != RedeemUnknown {
		t.Fatal("restart retained a live code")
	}
}
