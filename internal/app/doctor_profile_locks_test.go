package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func legacyProfileLockFixture(t *testing.T) (idle, held string, lease *state.Lease, preserved map[string]string) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	write := func(name, content string) string {
		t.Helper()
		path := filepath.Join(state.Dir(), name)
		if err := state.WritePrivateFile(path, []byte(content)); err != nil {
			t.Fatal(err)
		}
		return path
	}
	digest := strings.Repeat("a", 64)
	idle = write("lease-profile-"+digest+".json.lock", "")
	heldKey := "profile-" + strings.Repeat("b", 64) + ".json"
	var err error
	lease, err = state.TryLease(heldKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = lease.Close() })
	held = filepath.Join(state.Dir(), "lease-"+heldKey+".lock")
	preserved = map[string]string{
		write("participant-profiles/"+digest+".json", "old profile"):      "old profile",
		write("participant-profiles/v2/"+digest+".json", "new profile"):   "new profile",
		write("participant-profiles/v2/locks/lease-aa.lock", ""):          "",
		write("lease-profile-"+strings.Repeat("c", 64)+".json.lock", "x"): "x",
		write("lease-unrelated.lock", ""):                                 "",
		write("lease-profile-short.json.lock", ""):                        "",
	}
	linkTarget := write("unrelated-empty-file", "")
	sym := filepath.Join(state.Dir(), "lease-profile-"+strings.Repeat("d", 64)+".json.lock")
	if err := os.Symlink(linkTarget, sym); err != nil {
		t.Fatal(err)
	}
	hard := filepath.Join(state.Dir(), "lease-profile-"+strings.Repeat("e", 64)+".json.lock")
	if err := os.Link(linkTarget, hard); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{linkTarget, sym, hard} {
		preserved[path] = ""
	}
	return idle, held, lease, preserved
}

func TestSTOR011DoctorReportsLegacyProfileLocksWithoutChanges(t *testing.T) {
	idle, held, _, _ := legacyProfileLockFixture(t)
	var out bytes.Buffer
	if rc := writeDoctorJSON(&out, []dsection{{"Storage", []dcheck{rowLegacyProfileLocks()}}}, time.Unix(0, 0)); rc != 0 {
		t.Fatalf("exit=%d", rc)
	}
	var report doctorReport
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	row := report.Sections[0].Rows[0]
	if row.Status != "recommended" || row.Value != "2" || !strings.Contains(row.Note, "gtmux doctor --prune-profile-locks") {
		t.Fatalf("unexpected row: %+v", row)
	}
	for _, mode := range []string{"--json", "--no-prompt", "--fix", "--yes", "--terminal", "--bundle"} {
		if rc := cmdDoctor([]string{mode, "--prune-profile-locks"}); rc != 2 {
			t.Fatalf("%s combined with cleanup returned %d", mode, rc)
		}
	}
	for _, path := range []string{idle, held} {
		if _, err := os.Lstat(path); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSTOR012DoctorPrunesOnlyConfirmedIdleLegacyProfileLocks(t *testing.T) {
	idle, held, lease, preserved := legacyProfileLockFixture(t)
	if rc := doctorPruneProfileLocks(func(string) bool { return false }); rc != 0 {
		t.Fatalf("declined cleanup exit=%d", rc)
	}
	if _, err := os.Lstat(idle); err != nil {
		t.Fatal(err)
	}
	confirm := func(prompt string) bool {
		if !strings.Contains(prompt, "[y/N]") {
			t.Fatalf("expected explicit confirmation: %q", prompt)
		}
		return true
	}
	if rc := doctorPruneProfileLocks(confirm); rc != 1 || rowLegacyProfileLocks().value != "1" {
		t.Fatalf("expected one held lock remaining, exit=%d", rc)
	}
	if _, err := os.Lstat(held); err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	if rc := doctorPruneProfileLocks(confirm); rc != 0 || rowLegacyProfileLocks().status != stOK {
		t.Fatalf("retry did not clear the report, exit=%d", rc)
	}
	if rc := doctorPruneProfileLocks(func(string) bool {
		t.Fatal("asked for confirmation with no files to prune")
		return false
	}); rc != 0 {
		t.Fatalf("repeat cleanup exit=%d", rc)
	}
	for _, path := range []string{idle, held} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected %s removed, stat error=%v", path, err)
		}
	}
	for path, want := range preserved {
		if data, err := os.ReadFile(path); err != nil || string(data) != want {
			t.Fatalf("changed %s, read error=%v", path, err)
		}
	}
}
