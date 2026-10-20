package app

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/server"
)

func TestMemoryDiagnosticsSignalCleanup(t *testing.T) {
	if dir := os.Getenv("GTMUX_TEST_MEMORY_SIGNAL"); dir != "" {
		s := startMemoryDiagnostics(dir)
		if s == nil {
			t.Fatal("fixture listener did not start")
		}
		stopOnSignal(func() { _ = s.Close() })
		// Publish readiness only after the path is fully written. A reader can
		// observe an empty file between WriteFile's create and write operations.
		ready := filepath.Join(dir, "ready")
		if err := os.WriteFile(ready+".tmp", []byte(s.Path), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(ready+".tmp", ready); err != nil {
			t.Fatal(err)
		}
		select {}
	}
	// The child runs only the listener and existing signal hook, never cmdServe.
	// Redirect log storage before the hook runs; no real service or tmux is used.
	t.Setenv("HOME", t.TempDir())
	dir, err := os.MkdirTemp("/tmp", "gtmux-md-sig-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestMemoryDiagnosticsSignalCleanup$")
	cmd.Env = append(os.Environ(), "GTMUX_TEST_MEMORY_SIGNAL="+dir)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var socket []byte
	for ctx.Err() == nil {
		socket, err = os.ReadFile(filepath.Join(dir, "ready"))
		if err == nil && len(socket) > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ctx.Err() != nil {
		_ = cmd.Wait()
		t.Fatal("fixture did not become ready")
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		cancel()
		_ = cmd.Wait()
		t.Fatal(err)
	}
	_ = cmd.Wait()
	status, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGTERM {
		t.Fatalf("signal semantics changed: %v", cmd.ProcessState)
	}
	if _, err := os.Stat(filepath.Dir(string(socket))); !os.IsNotExist(err) {
		t.Fatalf("signal left diagnostic listener files for %q: %v", socket, err)
	}
}

func TestMemoryDiagnosticsAbsentFromApplicationRoutes(t *testing.T) {
	s := server.New(server.Config{Token: "fixture-owner"}, server.Deps{})
	for _, path := range []string{"/stats", "/heap", "/allocs", "/debug/pprof/heap", "/api/heap"} {
		for _, token := range []string{"", "fixture-owner"} {
			r := httptest.NewRequest("GET", path, nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			s.Handler().ServeHTTP(w, r)
			if w.Code != 404 || w.Header().Get("X-Gtmux-PID") != "" {
				t.Fatalf("memory route exposed on application handler: %s %d", path, w.Code)
			}
		}
	}
}

func TestMemoryDiagnosticFailureIsOptional(t *testing.T) {
	if s := startMemoryDiagnostics(""); s != nil {
		_ = s.Close()
		t.Fatal("default enabled diagnostics")
	}
	if s := startMemoryDiagnostics("relative/unsafe"); s != nil {
		_ = s.Close()
		t.Fatal("unsafe fallback listener")
	}
}

func TestMemoryDiagnosticsRejectServiceMutation(t *testing.T) {
	for _, args := range [][]string{
		{"--memory-diagnostics"}, {"--memory-diagnostics="},
		{"--memory-diagnostics", "/private/unused", "--service"},
		{"--unservice", "--memory-diagnostics=/private/unused"},
	} {
		if got := cmdServe(args); got != 2 {
			t.Fatalf("%v returned %d", args, got)
		}
	}
}
