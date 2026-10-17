package server

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/diag"
)

func TestAccessOriginUsesVerifiedIdentityBeforeTransport(t *testing.T) {
	for _, tc := range []struct {
		name, peer, token, actor, forwarded, origin, basis string
	}{
		{"menu bar", "127.0.0.1:10", "master", "menubar", "", diag.AccessLocal, "local_tool"},
		{"CLI", "127.0.0.1:10", "master", "user", "", diag.AccessLocal, "local_tool"},
		{"doctor", "[::1]:10", "master", "system", "", diag.AccessLocal, "local_tool"},
		{"unidentified master", "127.0.0.1:10", "master", "", "", diag.AccessUnknown, "unresolved"},
		{"forged actor on paired phone", "127.0.0.1:10", "phone-token", "menubar", "", diag.AccessExternal, "paired_client"},
		{"headerless browser proxy", "[::1]:10", "browser-token", "system", "", diag.AccessExternal, "paired_client"},
		{"LAN master", "192.168.1.2:10", "master", "menubar", "", diag.AccessExternal, "network_peer"},
		{"direct tunnel", "127.0.0.1:10", "master", "system", "203.0.113.7", diag.AccessExternal, "forwarded_peer"},
		{"standard tunnel", "127.0.0.1:10", "master", "menubar", "198.51.100.3, 203.0.113.7", diag.AccessExternal, "forwarded_peer"},
		{"unusable forwarding", "127.0.0.1:10", "master", "menubar", "not-an-IP", diag.AccessUnknown, "unresolved"},
		{"loopback forwarding", "127.0.0.1:10", "master", "menubar", "127.0.0.1", diag.AccessUnknown, "unresolved"},
		{"untrusted forwarding", "192.168.1.2:10", "master", "system", "127.0.0.1", diag.AccessExternal, "network_peer"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			enroll := NewEnrollManager([]EnrolledDevice{
				{ID: "full-phone", Token: "phone-token", Scope: scopeDevice, Name: "Phone"},
				{ID: "full-browser", Token: "browser-token", Scope: scopeDevice, Name: "Browser"},
			}, nil)
			s := New(Config{Token: "master"}, Deps{Enroll: enroll, AgentsJSON: func() ([]byte, error) { return []byte(`[]`), nil }})
			req := httptest.NewRequest("GET", "/api/agents", nil)
			req.RemoteAddr = tc.peer
			req.Header.Set("Authorization", "Bearer "+tc.token)
			req.Header.Set(ActorHeader, tc.actor)
			req.Header.Set("X-Forwarded-For", tc.forwarded)
			req.Header.Set("X-Gtmux-Client", "iPhone")
			req.Header.Set("X-Gtmux-Device-ID", "forged-device")
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, req)
			if w.Code != 200 {
				t.Fatal(w.Code)
			}
			all, err := diag.ReadAccess(diag.AccessQuery{Origin: diag.AccessAll})
			if err != nil || len(all.Entries) != 1 {
				t.Fatal(all, err)
			}
			e := all.Entries[0]
			if e.Origin != tc.origin || e.OriginBasis != tc.basis || e.DeviceID == "forged-device" {
				t.Fatalf("wrong provenance: %+v", e)
			}
			visible, err := diag.ReadAccess(diag.AccessQuery{})
			want := 0
			if tc.origin == diag.AccessExternal {
				want = 1
			}
			if err != nil || len(visible.Entries) != want || len(visible.Groups) != want {
				t.Fatal(visible, err)
			}
		})
	}
}

func TestExternalAccessFailuresAndSamplingAreIndependentOfInternalTraffic(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	s := New(Config{Token: "master"}, Deps{AgentsJSON: func() ([]byte, error) { return []byte(`[]`), nil }})
	call := func(peer, path string) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = peer
		r.Header.Set("Authorization", "Bearer master")
		r.Header.Set(ActorHeader, "menubar")
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	call("127.0.0.1:10", "/api/agents")
	call("203.0.113.8:10", "/api/agents")
	call("203.0.113.8:11", "/api/agents")
	s.deps.AgentsJSON = func() ([]byte, error) { return nil, errors.New("synthetic failure") }
	call("203.0.113.8:12", "/api/agents")
	call("203.0.113.8:13", "/api/agents")
	p, err := diag.ReadAccess(diag.AccessQuery{})
	if err != nil || len(p.Entries) != 3 || p.Groups[0].Count != 3 || p.Groups[0].Failed != 2 {
		t.Fatal(p, err)
	}
	// Test the rejection sampler in isolation so prior tests cannot consume its window.
	var rejected rejects
	for _, peer := range []string{"127.0.0.1:10", "203.0.113.9:10", "203.0.113.9:11"} {
		r := httptest.NewRequest("GET", "/api/agents", nil)
		r.RemoteAddr = peer
		rejected.note(r)
	}
	p, err = diag.ReadAccess(diag.AccessQuery{Outcome: diag.Refused})
	if err != nil || len(p.Entries) != 1 || p.Entries[0].Origin != diag.AccessExternal {
		t.Fatal(p, err)
	}
	// The HTTP endpoint must use the same external default and counts.
	w := call("127.0.0.1:10", "/api/access-history?group_by=device")
	var api diag.AccessPage
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &api) != nil || api.Query.Origin != diag.AccessExternal || len(api.Groups) != 2 || len(api.Entries) != 0 {
		t.Fatal(w.Code, w.Body.String())
	}
	if call("127.0.0.1:10", "/api/access-history?origin=invalid").Code != 400 {
		t.Fatal("invalid origin accepted")
	}
}

func TestForwardingHintsWithoutVerifiedSourceStayUnknown(t *testing.T) {
	for _, header := range []string{"Forwarded", "Cf-Ray", "Cf-Connecting-IP", "X-Forwarded-Proto"} {
		r := httptest.NewRequest("GET", "/api/agents", nil)
		r.RemoteAddr = "127.0.0.1:10"
		r.Header.Set(header, "unverified")
		if origin, _ := accessOrigin(r); origin != diag.AccessUnknown {
			t.Fatal(header, origin)
		}
	}
}

func TestPairedGuestRefusalRemainsExternalOverLoopback(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	enroll := NewEnrollManager([]EnrolledDevice{{ID: "full-guest", Token: "guest-secret", Scope: scopeGuest, Name: "Shared browser"}}, nil)
	s := New(Config{Token: "master"}, Deps{Enroll: enroll})
	r := httptest.NewRequest("GET", "/api/access-history", nil)
	r.RemoteAddr = "127.0.0.1:10"
	r.Header.Set("Authorization", "Bearer guest-secret")
	r.Header.Set(ActorHeader, "menubar")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	p, err := diag.ReadAccess(diag.AccessQuery{})
	if w.Code != 403 || err != nil || len(p.Entries) != 1 || p.Entries[0].DeviceID != "full-guest" || p.Entries[0].Outcome != diag.Refused {
		t.Fatal(w.Code, p, err)
	}
}
