package memdiag

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"
)

type directoryOwner struct {
	os.FileInfo
	uid uint32
}

func (d directoryOwner) Sys() any { return &syscall.Stat_t{Uid: d.uid} }

func TestParentOwnershipBoundary(t *testing.T) {
	info, err := os.Stat(privateDir(t))
	if err != nil {
		t.Fatal(err)
	}
	for _, uid := range []uint32{0, uint32(os.Geteuid())} {
		if !protectedParent(directoryOwner{FileInfo: info, uid: uid}) {
			t.Fatal("trusted owner rejected")
		}
	}
	if protectedParent(directoryOwner{FileInfo: info, uid: uint32(os.Geteuid() + 1)}) {
		t.Fatal("foreign owner can replace our socket subtree")
	}
}

func privateDir(t *testing.T) string {
	t.Helper()
	// Unix socket paths have a short platform limit; t.TempDir's test name can
	// exceed it on macOS before the socket name is added.
	d, err := os.MkdirTemp("/tmp", "gtmux-md-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

func TestDisabledAndUnsafeDirectories(t *testing.T) {
	before := runtime.MemProfileRate
	s, err := Start("", "test")
	if s != nil || err != nil || runtime.MemProfileRate != before {
		t.Fatal("disabled diagnostics changed state", s, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	d := privateDir(t)
	link := filepath.Join(d, "link")
	if err := os.Symlink(d, link); err != nil {
		t.Fatal(err)
	}
	public := filepath.Join(d, "public")
	if err := os.Mkdir(public, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(public, 0o755); err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(d, "shared")
	if err := os.Mkdir(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(shared, 0o777); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(shared, "child")
	if err := os.Mkdir(child, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"relative", filepath.Join(d, "missing"), link, public, child} {
		if s, err := Start(path, "test"); err == nil {
			_ = s.Close()
			t.Fatalf("unsafe path accepted: %s", path)
		}
	}
	info, _ := os.Stat(public)
	if info.Mode().Perm() != 0o755 {
		t.Fatal("failure changed caller directory")
	}
}

func TestListenFailureRemovesOnlyOwnRun(t *testing.T) {
	d := privateDir(t)
	long := filepath.Join(d, strings.Repeat("x", 100))
	if err := os.Mkdir(long, 0o700); err != nil {
		t.Fatal(err)
	}
	if s, err := Start(long, "test"); err == nil {
		_ = s.Close()
		t.Fatal("overlong Unix socket path accepted")
	}
	entries, err := os.ReadDir(long)
	if err != nil || len(entries) != 0 {
		t.Fatalf("failed listen left artifacts: %v %v", entries, err)
	}
}

func TestPrivateSocketStatsAndCleanup(t *testing.T) {
	d := privateDir(t)
	s, err := Start(d, "test-version")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	for path, want := range map[string]os.FileMode{s.dir: 0o700, s.Path: 0o600} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode().Perm() != want {
			t.Fatalf("permissions %s: %v %v", path, info, err)
		}
	}
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", s.Path)
	}}
	t.Cleanup(tr.CloseIdleConnections)
	client := &http.Client{Transport: tr, Timeout: 3 * time.Second}
	resp, err := client.Get("http://local/stats")
	if err != nil {
		t.Fatal(err)
	}
	var m Snapshot
	err = json.NewDecoder(resp.Body).Decode(&m)
	_ = resp.Body.Close()
	if err != nil || resp.StatusCode != 200 || m.PID != os.Getpid() || m.Version != "test-version" || m.HeapSys == 0 || m.NumGoroutine == 0 || m.CapturedAt.IsZero() {
		t.Fatalf("snapshot: %+v, status=%d err=%v", m, resp.StatusCode, err)
	}
	// Shutdown closes idle transports and removes this run, but keeps the base.
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.dir); !os.IsNotExist(err) {
		t.Fatalf("run directory remains: %v", err)
	}
	if _, err := os.Stat(d); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Get("http://local/stats"); err == nil {
		t.Fatal("closed socket still serves")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestClosePreservesSubstitutedFile(t *testing.T) {
	s, err := Start(privateDir(t), "test")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(s.Path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.Path, []byte("preserve"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err == nil {
		t.Fatal("socket replacement not reported")
	}
	b, err := os.ReadFile(s.Path)
	if err != nil || string(b) != "preserve" {
		t.Fatal("replacement removed", err)
	}
}

func request(h http.Handler, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestProfilesAreGzipProtobufAndDoNotForceGC(t *testing.T) {
	for _, name := range []string{"heap", "allocs"} {
		h := newHandler("test")
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		w := request(h, "/"+name)
		runtime.ReadMemStats(&after)
		if w.Code != 200 || w.Header().Get("X-Gtmux-PID") == "" || w.Header().Get("X-Gtmux-Captured-At") == "" {
			t.Fatalf("profile: %d %s", w.Code, w.Body)
		}
		z, err := gzip.NewReader(bytes.NewReader(w.Body.Bytes()))
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(io.LimitReader(z, 32<<20))
		if err != nil {
			t.Fatal(err)
		}
		_ = z.Close()
		for _, typ := range []string{"alloc_space", "inuse_space", "inuse_objects"} {
			if !bytes.Contains(data, []byte(typ)) {
				t.Fatalf("profile lacks sample type %s", typ)
			}
		}
		if after.NumForcedGC != before.NumForcedGC {
			t.Fatal("capture forced a GC")
		}
		if request(h, "/"+name).Code != 429 {
			t.Fatal("profile cooldown missing")
		}
		if request(h, "/stats").Code != 200 {
			t.Fatal("cooldown blocked counters")
		}
	}
}

func TestRequestBoundaries(t *testing.T) {
	h := newHandler("test")
	for _, path := range []string{"/heap?gc=1", "/heap?seconds=1", "/stats?", "/stats?debug=1"} {
		if request(h, path).Code != 400 {
			t.Fatalf("accepted query: %s", path)
		}
	}
	for _, path := range []string{"/debug/pprof/", "/goroutine", "/profile", "/trace", "/cmdline"} {
		if request(h, path).Code != 404 {
			t.Fatalf("unexpected diagnostic route: %s", path)
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/stats", strings.NewReader("secret")))
	if w.Code != 400 || strings.Contains(w.Body.String(), "secret") {
		t.Fatal("body accepted or echoed")
	}
}

func TestProfileBudgetsFailureCancellationAndConcurrency(t *testing.T) {
	t.Run("bucket metadata", func(t *testing.T) {
		h := newHandler("test")
		h.memstats = func(m *runtime.MemStats) { m.BuckHashSys = maxProfileBuckets + 1 }
		h.records = func() int { t.Fatal("over-budget metadata traversed"); return 0 }
		h.profile = func(io.Writer, string) error { t.Fatal("over-budget profile generated"); return nil }
		if request(h, "/heap").Code != 413 || request(h, "/stats").Code != 200 {
			t.Fatal("metadata budget must refuse profiles and retain counters")
		}
	})
	t.Run("records", func(t *testing.T) {
		h := newHandler("test")
		h.records = func() int { return maxProfileRecords + 1 }
		h.profile = func(io.Writer, string) error { t.Fatal("over-budget profile generated"); return nil }
		if request(h, "/heap").Code != 413 {
			t.Fatal("record budget missing")
		}
	})
	for _, tc := range []struct {
		name   string
		write  func(io.Writer, string) error
		status int
	}{
		{"encoded size", func(w io.Writer, _ string) error { _, err := w.Write(make([]byte, maxProfileBytes+1)); return err }, 413},
		{"failure", func(w io.Writer, _ string) error {
			_, _ = w.Write([]byte("partial private bytes"))
			return errors.New("private error")
		}, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHandler("test")
			h.profile = tc.write
			w := request(h, "/heap")
			if w.Code != tc.status || strings.Contains(w.Body.String(), "private") {
				t.Fatalf("partial profile leaked: status=%d bytes=%d", w.Code, w.Body.Len())
			}
			if request(h, "/stats").Code != 200 {
				t.Fatal("failure disabled counters")
			}
		})
	}
	t.Run("cancel and busy", func(t *testing.T) {
		h := newHandler("test")
		entered, release, done := make(chan struct{}), make(chan struct{}), make(chan struct{})
		h.profile = func(w io.Writer, _ string) error {
			close(entered)
			<-release
			_, err := w.Write([]byte("cancelled profile"))
			return err
		}
		ctx, cancel := context.WithCancel(context.Background())
		w := httptest.NewRecorder()
		go func() {
			h.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/heap", nil).WithContext(ctx))
			close(done)
		}()
		<-entered
		if request(h, "/allocs").Code != 429 || request(h, "/stats").Code != 429 {
			t.Fatal("busy capture queued more work")
		}
		cancel()
		close(release)
		<-done
		if strings.Contains(w.Body.String(), "cancelled profile") {
			t.Fatal("cancelled profile delivered")
		}
		if request(h, "/stats").Code != 200 {
			t.Fatal("cancel kept lock")
		}
	})
}
