package server

// The endpoint's own behaviour. The guest refusal lives with the other HQ surfaces in
// hq_test.go, so a new one added without a scope check fails THERE rather than needing
// somebody to remember this file.

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestHQMemoryStreamsAnArchiveToTheOwner(t *testing.T) {
	s := hqTestServer(t, Deps{HQMemory: func(w io.Writer) (int64, error) {
		gz := gzip.NewWriter(w)
		tw := tar.NewWriter(gz)
		body := []byte("## board")
		_ = tw.WriteHeader(&tar.Header{Name: "notes/board.md", Size: int64(len(body)), Mode: 0o644})
		n, _ := tw.Write(body)
		_ = tw.Close()
		_ = gz.Close()
		return int64(n), nil
	}})
	w := hqGet(t, s, "/api/hq/memory", "master")

	if w.Code != http.StatusOK {
		t.Fatalf("owner got %d, want 200", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/gzip" {
		t.Errorf("Content-Type = %q", ct)
	}
	// A filename, so what lands on the phone is not "memory" among a hundred others.
	if cd := w.Header().Get("Content-Disposition"); !strings.Contains(cd, "gtmux-hq-") || !strings.Contains(cd, ".tar.gz") {
		t.Errorf("Content-Disposition = %q", cd)
	}
	// NO Content-Length, and this is the real guard for a stream that can fail
	// halfway: the archive is produced as it is written, so a length declared up front
	// would be a guess, and a client that trusted it would truncate a good backup and
	// believe it. (That a CUT stream is detectable is a property of chunked encoding,
	// not of this handler — httptest's recorder cannot model a dropped connection, so
	// there is no test for it here rather than one that passes for the wrong reason.)
	if cl := w.Header().Get("Content-Length"); cl != "" {
		t.Errorf("Content-Length = %q — it cannot be known before the archive is built", cl)
	}
	gz, err := gzip.NewReader(strings.NewReader(w.Body.String()))
	if err != nil {
		t.Fatalf("the body is not a gzip stream: %v", err)
	}
	h, err := tar.NewReader(gz).Next()
	if err != nil || h.Name != "notes/board.md" {
		t.Errorf("archive head = %+v, err %v", h, err)
	}
}

func TestHQMemoryOnAMachineWithNoSupervisor(t *testing.T) {
	s := hqTestServer(t, Deps{})
	w := hqGet(t, s, "/api/hq/memory", "master")
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want 404 when there is nothing to export", w.Code)
	}
}
