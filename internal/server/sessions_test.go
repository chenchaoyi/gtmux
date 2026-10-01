package server

import (
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestCreateSessionScopeValidationAndReceipt(t *testing.T) {
	calls := 0
	deps := Deps{CreateSession: func(name, id string) (SessionCreated, error) {
		calls++
		return SessionCreated{"work", "%7", "0", "0", "work:0.0"}, nil
	}}
	h := New(Config{Token: testToken}, deps).Handler()
	body := `{"name":"work","request_id":"request-1234567890"}`
	if r := post(t, h, "/api/sessions", "bad-token", body); r.Code != 401 {
		t.Fatalf("auth %d", r.Code)
	}
	// Enrolled owner devices have full scope, guests never do, even with input grants.
	em := NewEnrollManager(nil, nil)
	code := em.Mint()
	owner, _ := em.Redeem(code, "iPhone")
	guest := em.MintGuest("colleague", []string{"%7"}, []string{"%7"}, 0)
	deps.Enroll = em
	h = New(Config{Token: testToken}, deps).Handler()
	if r := post(t, h, "/api/sessions", guest.Token, body); r.Code != 403 {
		t.Fatalf("guest %d", r.Code)
	}
	if calls != 0 {
		t.Fatal("unauthorized creation happened")
	}
	if r := post(t, h, "/api/sessions", owner.Token, body); r.Code != 200 || !strings.Contains(r.Body.String(), `"pane_id":"%7"`) {
		t.Fatalf("owner %d %s", r.Code, r.Body.String())
	}
	for _, bad := range []string{`{}`, `{"request_id":"short"}`, `{"request_id":"request-1234567890","command":"rm -rf"}`, body + body, `{"name":"a\nb","request_id":"request-1234567890"}`} {
		before := calls
		r := post(t, h, "/api/sessions", testToken, bad)
		if r.Code != 400 || calls != before {
			t.Fatalf("invalid request ran: %s %d", bad, r.Code)
		}
	}
	if r := do(t, h, http.MethodGet, "/api/sessions", testToken); r.Code != 405 {
		t.Fatal(r.Code)
	}
}

func TestCreateSessionFailuresRemainHonest(t *testing.T) {
	for _, tt := range []struct {
		err    error
		status int
		code   string
	}{{&SessionCreateError{"name_exists"}, 409, "name_exists"}, {errors.New("tmux failed"), 503, "create_failed"}} {
		h := New(Config{Token: testToken}, Deps{CreateSession: func(string, string) (SessionCreated, error) { return SessionCreated{}, tt.err }}).Handler()
		r := post(t, h, "/api/sessions", testToken, `{"request_id":"request-1234567890"}`)
		if r.Code != tt.status || !strings.Contains(r.Body.String(), tt.code) {
			t.Fatalf("%d %s", r.Code, r.Body.String())
		}
	}
	h := New(Config{Token: testToken}, Deps{}).Handler()
	if r := post(t, h, "/api/sessions", testToken, `{}`); r.Code != 501 {
		t.Fatal(r.Code)
	}
}
