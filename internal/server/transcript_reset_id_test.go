package server

import (
	"net/http"
	"testing"
)

func TestTranscriptResetGenerationOnEmptyBody(t *testing.T) {
	meta := TranscriptMeta{Reset: "clear", ResetID: "first"}
	s := hqTestServer(t, Deps{Transcript: func(string, int) ([]byte, TranscriptMeta, error) {
		return []byte("[]"), meta, nil
	}})
	check := func(want string) {
		t.Helper()
		w := hqGet(t, s, "/api/transcript?id=%251", "master")
		if w.Code != http.StatusOK || w.Body.String() != "[]" || w.Header().Get("X-Gtmux-Session-Reset-ID") != want {
			t.Fatalf("unexpected empty transcript response: %d, %s, %v", w.Code, w.Body, w.Header())
		}
	}
	check("first")
	meta.ResetID = "second"
	check("second")
	check("second")
	meta.Reset = ""
	check("")
}
