package server

import (
	"bytes"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/artifact"
)

type documentReader struct{ *bytes.Reader }

func (documentReader) Close() error { return nil }

func TestDocumentScopeIsCheckedBeforeAnyLookup(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	enroll := NewEnrollManager(nil, nil)
	guest := enroll.MintGuest("guest", []string{"%1"}, []string{"%1"}, 0)
	calls := 0
	s := New(Config{Token: testToken}, Deps{Enroll: enroll,
		ArtifactResolve: func(_, _, _ string) (artifact.File, error) { calls++; return artifact.File{}, nil },
		ArtifactRelated: func(_, _, _, _ string) (artifact.File, error) { calls++; return artifact.File{}, nil },
		ArtifactContent: func(_, _, _ string) (artifact.File, io.ReadSeekCloser, error) {
			calls++
			return artifact.File{}, documentReader{bytes.NewReader(nil)}, nil
		},
	})
	for _, token := range []string{"", guest.Token} {
		for _, path := range []string{"/api/artifacts/resolve", "/api/artifacts/content"} {
			r := httptest.NewRequest("GET", path, nil)
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 401 && w.Code != 403 {
				t.Fatalf("scope accepted: %d", w.Code)
			}
		}
	}
	if calls != 0 {
		t.Fatal("unauthorized caller reached the document store")
	}
}

func TestDocumentDownloadNeverRendersHTMLOnTheAPIOrigin(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	const content = "<html><script>parent.localStorage.clear()</script></html>"
	meta := artifact.File{ID: strings.Repeat("a", 32), Name: "report.html", Kind: "html", MediaType: "text/html", Bytes: int64(len(content)), SHA256: strings.Repeat("b", 64), SessionID: "session"}
	s := New(Config{Token: testToken}, Deps{
		ArtifactResolve: func(ref, sid, home string) (artifact.File, error) {
			if ref != "gtmux-file:"+meta.ID || sid != "session" || home != "" {
				t.Fatal("reference was changed")
			}
			return meta, nil
		},
		ArtifactContent: func(_, _, _ string) (artifact.File, io.ReadSeekCloser, error) {
			return meta, documentReader{bytes.NewReader([]byte(content))}, nil
		},
	})
	r := httptest.NewRequest("POST", "/api/artifacts/resolve", strings.NewReader(`{"reference":"gtmux-file:`+meta.ID+`","session_id":"session"}`))
	r.Header.Set("Authorization", "Bearer "+testToken)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || !strings.Contains(w.Body.String(), meta.ID) {
		t.Fatal("metadata unavailable", w.Code)
	}
	r = httptest.NewRequest("GET", "/api/artifacts/content?id="+meta.ID+"&session_id=session", nil)
	r.Header.Set("Authorization", "Bearer "+testToken)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != 200 || w.Body.String() != content || w.Header().Get("Content-Type") != "application/octet-stream" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") || w.Header().Get("Content-Security-Policy") != "sandbox; default-src 'none'" || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("active content escaped the download boundary", w.Code, w.Header())
	}
}

func TestRelatedDocumentLookupUsesParentAndRejectsInvalidRequests(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	calls := 0
	s := New(Config{Token: testToken}, Deps{ArtifactRelated: func(ref, parent, sid, home string) (artifact.File, error) {
		calls++
		if ref != "images/shot.png" || parent != strings.Repeat("a", 32) || sid != "session" || home != "" {
			t.Fatal("identity changed", ref, parent, sid, home)
		}
		return artifact.File{ID: strings.Repeat("b", 32)}, nil
	}})
	for _, body := range []string{
		`{"reference":"images/shot.png","session_id":"session","parent_id":"bad"}`,
		`{"reference":"images/shot.png","session_id":"session","parent_id":"` + strings.Repeat("a", 32) + `","unexpected":true}`,
	} {
		request := httptest.NewRequest("POST", "/api/artifacts/resolve", strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+testToken)
		response := httptest.NewRecorder()
		s.Handler().ServeHTTP(response, request)
		if response.Code != 400 {
			t.Fatal(response.Code, response.Body.String())
		}
	}
	if calls != 0 {
		t.Fatal("invalid input reached source lookup")
	}
	request := httptest.NewRequest("POST", "/api/artifacts/resolve", strings.NewReader(`{"reference":"images/shot.png","session_id":"session","parent_id":"`+strings.Repeat("a", 32)+`"}`))
	request.Header.Set("Authorization", "Bearer "+testToken)
	response := httptest.NewRecorder()
	s.Handler().ServeHTTP(response, request)
	if response.Code != 200 || calls != 1 {
		t.Fatal(response.Code, response.Body.String(), calls)
	}
}
