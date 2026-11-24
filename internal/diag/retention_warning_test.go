//go:build darwin || linux

package diag

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
)

func TestRetentionMarkerRearmsDiskFullWarning(t *testing.T) {
	dir := setup(t, day1)
	t.Setenv("GTMUX_DEBUG", "bridge")
	path := DayFile(dir, "2026-09-19")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Truncate(DebugDropAt); err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	fullWarnings := 0
	restore := routeStoreFailuresToSystemLog(func(message string) bool {
		if strings.Contains(message, syscall.ENOSPC.Error()) {
			fullWarnings++
		}
		return true
	})
	t.Cleanup(restore)
	ReportStoreFailure("diagnostics", syscall.ENOSPC)
	appendLine(dir, Entry{Component: "log", Event: "fixture.blocked"})
	ReportStoreFailure("diagnostics", syscall.ENOSPC)
	if fullWarnings != 1 {
		t.Fatalf("failed internal append re-armed the warning, count=%d", fullWarnings)
	}

	For("bridge").Debug("fixture.debug", "dropped")
	f, err = os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tail := io.NewSectionReader(f, DebugDropAt, MaxEntry+1)
	var entry Entry
	decoder := json.NewDecoder(tail)
	if err := decoder.Decode(&entry); err != nil || entry.Event != "log.debug.dropped" {
		t.Fatalf("on-disk debug ceiling marker = %q, %v", entry.Event, err)
	}
	if err := decoder.Decode(&entry); err != io.EOF {
		t.Fatalf("unexpected entry after the debug ceiling marker: %v", err)
	}
	ReportStoreFailure("diagnostics", syscall.ENOSPC)
	if fullWarnings != 2 {
		t.Fatalf("full-volume warnings after a successful retention append = %d, want 2", fullWarnings)
	}
}
