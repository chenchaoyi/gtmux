package state

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestCHH002_AtomicMarkersAndWaitingSetIgnorePrivateTemps(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	marker := WaitingPath("%7")
	if err := WriteMarker(marker, "approval"); err != nil {
		t.Fatalf("WriteMarker: %v", err)
	}
	if got := ReadMarker(marker); got != "approval" {
		t.Fatalf("ReadMarker = %q, want approval", got)
	}
	info, err := os.Lstat(marker)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("marker mode = %v err=%v, want 0600", info.Mode().Perm(), err)
	}

	// Temporary private staging files must never appear as waiting pane IDs.
	tmpStage := filepath.Join(WaitingDir(), privateTempPrefix+"12345")
	if err := os.WriteFile(tmpStage, []byte("partial"), 0o600); err != nil {
		t.Fatal(err)
	}
	ws := WaitingSet()
	if !ws["%7"] || ws[filepath.Base(tmpStage)] {
		t.Fatalf("WaitingSet = %+v, want only %%7", ws)
	}

	numPath := StorePath("mine", "last-pass")
	if err := WriteInt64Marker(numPath, 987654321); err != nil {
		t.Fatalf("WriteInt64Marker: %v", err)
	}
	if got := ReadInt64Marker(numPath); got != 987654321 {
		t.Fatalf("ReadInt64Marker = %d, want 987654321", got)
	}
}

func TestCHH008_InspectPrivateTreeAllowsManagedCodexFrontendSymlinkOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	root := RootStorePath("codex-frontends")
	clientDir := filepath.Join(root, "client-12345")
	if err := MkdirAll(clientDir, 0o700); err != nil {
		t.Fatal(err)
	}

	evalTmp, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		t.Fatal(err)
	}
	daemonTarget := filepath.Join(evalTmp, "codex-daemon-"+strconv.Itoa(os.Geteuid()), "0123456789abcdef")
	sockAlias := filepath.Join(clientDir, "frontend.sock")
	if err := os.Symlink(daemonTarget, sockAlias); err != nil {
		t.Fatal(err)
	}

	report := InspectPrivateTree(Dir())
	if report.Wide != 0 || report.Unverified != 0 {
		t.Fatalf("valid codex frontend symlink flagged: %+v", report)
	}

	// A symlink pointing outside codex-daemon-<uid> must still be flagged as unverified.
	badClient := filepath.Join(root, "client-bad")
	if err := MkdirAll(badClient, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/tmp/foreign-socket", filepath.Join(badClient, "frontend.sock")); err != nil {
		t.Fatal(err)
	}
	reportBad := InspectPrivateTree(Dir())
	if reportBad.Unverified != 1 {
		t.Fatalf("expected foreign-target symlink to be flagged unverified, got %+v", reportBad)
	}
}
