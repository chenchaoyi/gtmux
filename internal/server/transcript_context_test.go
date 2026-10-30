package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTranscriptContextOwnerPairedGuestAndConditionalScope(t *testing.T) {
	enroll := NewEnrollManager(nil, nil)
	guest := enroll.MintGuest("viewer", []string{"%1"}, nil, 0)
	paired, ok := enroll.Redeem(enroll.Mint(), "phone")
	if !ok {
		t.Fatal("fixture")
	}
	body := []byte(`[{"prompt":"human","response":"public","future":7,"segments":[{"context":{"title":"Workspace instructions","text":"OWNER-CONTEXT","source":"fixture"}},{"text":"public","future_segment":true}]}]`)
	s := New(Config{Token: "master"}, Deps{Enroll: enroll, Transcript: func(string, int) ([]byte, TranscriptMeta, error) { return body, TranscriptMeta{Etag: `W/"one"`}, nil }}).Handler()
	read := func(token, etag string, include ...bool) *httptest.ResponseRecorder {
		path := "/api/transcript?id=%251"
		if len(include) == 0 || include[0] {
			path += "&context=1"
		}
		r := httptest.NewRequest(http.MethodGet, path, nil)
		r.Header.Set("Authorization", "Bearer "+token)
		r.Header.Set("If-None-Match", etag)
		w := httptest.NewRecorder()
		s.ServeHTTP(w, r)
		return w
	}
	owner := read("master", "")
	if owner.Code != 200 || !strings.Contains(owner.Body.String(), "OWNER-CONTEXT") {
		t.Fatal(owner)
	}
	phone := read(paired.Token, "")
	if phone.Code != 200 || !strings.Contains(phone.Body.String(), "OWNER-CONTEXT") {
		t.Fatal(phone)
	}
	public := read(guest.Token, owner.Header().Get("ETag"))
	if public.Code != 200 || strings.Contains(public.Body.String(), "OWNER-CONTEXT") || !strings.Contains(public.Body.String(), `"future":7`) || !strings.Contains(public.Body.String(), "public") {
		t.Fatal(public)
	}
	if public.Header().Get("ETag") == owner.Header().Get("ETag") {
		t.Fatal("scope reused an ETag")
	}
	if cached := read(guest.Token, public.Header().Get("ETag")); cached.Code != 304 || cached.Body.Len() != 0 {
		t.Fatal(cached)
	}
	if reverse := read("master", public.Header().Get("ETag")); reverse.Code != 200 || !strings.Contains(reverse.Body.String(), "OWNER-CONTEXT") {
		t.Fatal(reverse)
	}
	legacy := read("master", owner.Header().Get("ETag"), false)
	if legacy.Code != 200 || strings.Contains(legacy.Body.String(), "OWNER-CONTEXT") || strings.Contains(legacy.Body.String(), `"segments":[{}`) {
		t.Fatal("old reader received context or an empty segment", legacy)
	}
	if cached := read("master", legacy.Header().Get("ETag"), false); cached.Code != 304 {
		t.Fatal(cached)
	}
	if upgraded := read("master", legacy.Header().Get("ETag")); upgraded.Code != 200 || !strings.Contains(upgraded.Body.String(), "OWNER-CONTEXT") {
		t.Fatal(upgraded)
	}
}
func TestMalformedContextCannotFallThroughToGuest(t *testing.T) {
	_, _, err := traceContextBody([]byte(`[{"segments":{"context":"OWNER-CONTEXT"}}]`), TranscriptMeta{}, false)
	if err == nil {
		t.Fatal("malformed context must fail closed")
	}
}
