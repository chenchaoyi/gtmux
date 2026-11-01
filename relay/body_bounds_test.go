package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRelayMissingCredentialFailsClosedWithHealthAvailable(t *testing.T) {
	p := &fakePusher{}
	h := (&relayServer{pushers: map[string]Pusher{"ios": p}}).handler()
	if rr := post(t, h, `{"token":"fixture"}`, ""); rr.Code != 503 || len(p.got) > 0 {
		t.Fatalf("unconfigured relay reached provider: %d", rr.Code)
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/health", nil))
	if rr.Code != 200 {
		t.Fatal(rr.Code)
	}
}
func TestRelayRejectsLargeOrTrailingBodyBeforeProvider(t *testing.T) {
	p := &fakePusher{}
	h := (&relayServer{token: "fixture-secret", pushers: map[string]Pusher{"ios": p}}).handler()
	for _, tc := range []struct {
		name, body string
		length     int64
		code       int
	}{
		{"declared overflow", `{"token":"fixture"}`, 65537, 413},
		{"chunked overflow", `{"token":"fixture","body":"` + strings.Repeat("a", 65536) + `"}`, -1, 413},
		{"trailing padding overflow", `{"token":"fixture"}` + strings.Repeat(" ", 65536), -1, 413},
		{"second JSON document", `{"token":"fixture"} {}`, -1, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/push", strings.NewReader(tc.body))
			r.ContentLength = tc.length
			r.Header.Set("Authorization", "Bearer fixture-secret")
			rr := httptest.NewRecorder()
			h.ServeHTTP(rr, r)
			if rr.Code != tc.code {
				t.Fatalf("status %d want%d", rr.Code, tc.code)
			}
			if len(p.got) != 0 {
				t.Fatal("rejected body reached provider")
			}
		})
	}
	valid := `{"token":"fixture"}`
	valid += strings.Repeat(" ", 65536-len(valid))
	if rr := post(t, h, valid, "fixture-secret"); rr.Code != 200 || len(p.got) != 1 || p.got[0].Token != "fixture" {
		t.Fatalf("boundary request failed: %d", rr.Code)
	}
}
func TestRelayAuthenticatesBeforeOpeningBody(t *testing.T) {
	h := (&relayServer{token: "fixture-secret"}).handler()
	r := httptest.NewRequest("POST", "/push", strings.NewReader(strings.Repeat("x", 65537)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, r)
	if rr.Code != http.StatusUnauthorized {
		t.Fatal(rr.Code)
	}
}
