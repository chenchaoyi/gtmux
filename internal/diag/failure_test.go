//go:build darwin || linux

package diag

import (
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestInteractiveStoreFailureLeavesStderrAlone(t *testing.T) {
	dir := setup(t, day1)
	segment := DayFile(dir, "2026-09-19")
	if err := os.Mkdir(segment, 0o700); err != nil {
		t.Fatal(err)
	}
	readEnd, writeEnd, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = writeEnd.Close()
		_ = readEnd.Close()
	})
	old := os.Stderr
	os.Stderr = writeEnd
	t.Cleanup(func() { os.Stderr = old })

	var warnings []string
	for _, available := range []bool{true, false} {
		restore := routeStoreFailuresToSystemLog(func(message string) bool {
			warnings = append(warnings, message)
			return available
		})
		For("bridge").Info("fixture.write", "the private entry body")
		restore()
	}
	failureReport.Lock()
	delete(failureReport.last, "foreground")
	failureReport.Unlock()
	ReportStoreFailure("foreground", syscall.ENOSPC)
	if err := writeEnd.Close(); err != nil {
		t.Fatal(err)
	}
	stderr, err := io.ReadAll(readEnd)
	if err != nil {
		t.Fatal(err)
	}
	wantStderr := "gtmux: foreground store write failed: " + syscall.ENOSPC.Error() + "; run gtmux doctor\n"
	if string(stderr) != wantStderr {
		t.Fatalf("terminal output = %q, want only the foreground warning", stderr)
	}
	if len(warnings) != 2 {
		t.Fatalf("system log attempts = %d, want one in each bridge", len(warnings))
	}
	for _, warning := range warnings {
		if !strings.HasPrefix(warning, "diagnostics store write failed: ") ||
			strings.Contains(warning, dir) || strings.Contains(warning, "private entry body") {
			t.Fatalf("unexpected system warning: %q", warning)
		}
	}
}

func TestInteractiveDiskFullWarningRearmsAfterWrite(t *testing.T) {
	dir := setup(t, day1)
	var warnings []string
	logAvailable := false
	restore := routeStoreFailuresToSystemLog(func(message string) bool {
		warnings = append(warnings, message)
		return logAvailable
	})
	t.Cleanup(restore)
	route := systemWarnings.Load()
	full := &os.PathError{Op: "write", Path: filepath.Join(dir, "private.jsonl"), Err: syscall.ENOSPC}
	start := day1
	route.report("diagnostics", full, start)
	logAvailable = true
	route.report("diagnostics", full, start.Add(time.Second))
	route.report("diagnostics", full, start.Add(time.Minute))
	route.report("diagnostics", full, start.Add(2*time.Minute))
	route.report("events", syscall.EACCES, start.Add(2*time.Minute))
	For("bridge").Info("fixture.recovery", "append after storage recovery")
	if entries := readEntries(t, DayFile(dir, "2026-09-19")); len(entries) != 1 {
		t.Fatalf("recovery append count = %d, want 1", len(entries))
	}
	route.report("diagnostics", full, start.Add(2*time.Minute))

	wantFull := "diagnostics store write failed: " + syscall.ENOSPC.Error() + "; run gtmux doctor"
	wantEvents := "events store write failed: " + syscall.EACCES.Error() + "; run gtmux doctor"
	want := []string{wantFull, wantFull, wantEvents, wantFull}
	if !reflect.DeepEqual(warnings, want) {
		t.Fatalf("warning attempts = %q, want %q", warnings, want)
	}
}

func TestSystemStoreWarningUsesLocalSocket(t *testing.T) {
	// Keep the address short enough for Darwin Unix sockets.
	dir, err := os.MkdirTemp("/tmp", "gtx")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	path := filepath.Join(dir, "syslog")
	listener, err := net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	const message = "diagnostics store write failed: fixture; run gtmux doctor"
	if !sendSystemStoreWarning(message, []string{path}) {
		t.Fatal("system warning did not reach the local socket")
	}
	if err := listener.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	var b [1024]byte
	n, err := listener.Read(b[:])
	if err != nil {
		t.Fatal(err)
	}
	packet := string(b[:n])
	if !strings.HasPrefix(packet, "<12>") || !strings.Contains(packet, " gtmux[") ||
		!strings.HasSuffix(packet, "]: "+message+"\n") {
		t.Fatalf("unexpected system-log packet: %q", packet)
	}
	if sendSystemStoreWarning(message, []string{filepath.Join(dir, "missing")}) {
		t.Fatal("unavailable system log reported success")
	}
}
