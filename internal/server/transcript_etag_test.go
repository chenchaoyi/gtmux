package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The chat view polls while it is open, so the unchanged case must be cheap: an
// ETag on every response, and a 304 with no body when the caller presents it back.
func TestTranscriptConditionalGET(t *testing.T) {
	h := New(Config{Addr: "127.0.0.1:0", Token: testToken}, Deps{
		Transcript: func(string) ([]byte, TranscriptMeta, error) {
			return []byte(`[{"prompt":"hi"}]`), TranscriptMeta{Etag: `W/"sess-42"`, Dropped: 3}, nil
		},
	}).Handler()

	rr := do(t, h, http.MethodGet, "/api/transcript?id=%251", testToken)
	if rr.Code != http.StatusOK {
		t.Fatalf("first GET = %d, want 200", rr.Code)
	}
	tag := rr.Header().Get("ETag")
	if tag != `W/"sess-42"` {
		t.Fatalf("ETag = %q, want the validator the meta carried", tag)
	}

	rr2 := doIfNoneMatch(t, h, "/api/transcript?id=%251", tag)
	if rr2.Code != http.StatusNotModified {
		t.Fatalf("revalidation = %d, want 304", rr2.Code)
	}
	if rr2.Body.Len() != 0 {
		t.Fatalf("304 carried a %d-byte body", rr2.Body.Len())
	}
	// A 304 must still say the history is truncated: the client learned that from the
	// 200 and would otherwise lose it the moment the conversation goes quiet.
	if got := rr2.Header().Get("X-Gtmux-Turns-Dropped"); got != "3" {
		t.Fatalf("304 dropped-count header = %q, want 3", got)
	}

	// A stale validator gets the body, not a 304.
	rr3 := doIfNoneMatch(t, h, "/api/transcript?id=%251", `W/"sess-41"`)
	if rr3.Code != http.StatusOK || rr3.Body.Len() == 0 {
		t.Fatalf("stale validator = %d with %d bytes, want 200 with a body", rr3.Code, rr3.Body.Len())
	}
}

// doIfNoneMatch is `do` with a validator — the revalidating request a polling client
// makes on every tick once it has been served once.
func doIfNoneMatch(t *testing.T, h http.Handler, target, tag string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	req.Header.Set("If-None-Match", tag)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}
