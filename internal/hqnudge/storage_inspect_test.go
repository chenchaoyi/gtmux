package hqnudge

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSW004WakeStorageInventory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fixtures := map[string]string{
		"hq-nudges/10.p1.txt":                                        "not parsed",
		"hq-nudges/20.p1.txt.refused":                                "retained refusal",
		"hq-nudges/21.p1.hqrelay.refused":                            "retained relay refusal",
		"hq-nudges/30.p1.txt.sending":                                "claimed",
		"hq-nudges/30.p1.txt.stuck":                                  "another claim in the same slot",
		"hq-nudges/30.p1.txt.native-uncertain":                       "unknown",
		"hq-nudges/40.p1.txt.legacy-uncertain":                       "unknown",
		"hq-nudges/41.p1.a1.txt":                                     "unconfirmed legacy attempt",
		"hq-nudges/50.p1.txt.sending":                                "hard-link publication in progress",
		"hq-nudges/60.p1.r1-123-" + strings.Repeat("a", 32) + ".txt": "scheduled retry",
		"hq-nudges/61.p1.r1-invalid.txt":                             "unrecognized retry",
		"hq-nudges/not-a-wake.refused":                               "unrecognized suffix lookalike",
		"hq-nudges/foreign-file":                                     "unclassified",
		"hq-wake-accepted/receipt.json":                              "not parsed",
	}
	for relative, body := range fixtures {
		if err := state.WritePrivateFile(filepath.Join(state.Dir(), relative), []byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := state.ChangeStorageLayout(true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := state.ChangeWorkflowLayout(true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(state.WorkflowPath("hq-nudges", "50.p1.txt.sending"), state.WorkflowPath("hq-nudges", "50.p1.txt.native-uncertain")); err != nil {
		t.Fatal(err)
	}
	fixtures["hq-nudges/50.p1.txt.native-uncertain"] = fixtures["hq-nudges/50.p1.txt.sending"]
	symlink := state.WorkflowPath("hq-nudges", "70.p1.txt.refused")
	if err := os.Symlink("20.p1.txt.refused", symlink); err != nil {
		t.Fatal(err)
	}
	before := make(map[string]os.FileInfo, len(fixtures))
	for relative := range fixtures {
		path := state.WorkflowPath(filepath.Dir(relative), filepath.Base(relative))
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		before[relative] = info
	}
	usage, err := InspectStorage()
	if want := (StorageUsage{Queued: 2, Claimed: 3, Refused: 2, Uncertain: 4, AwaitingProof: 1, Unclassified: 4, ConflictingSlots: 1}); err != nil || usage != want {
		t.Fatalf("usage %+v, want %+v (%v)", usage, want, err)
	}
	for relative, body := range fixtures {
		path := state.WorkflowPath(filepath.Dir(relative), filepath.Base(relative))
		after, err := os.Lstat(path)
		if err != nil || !os.SameFile(before[relative], after) {
			t.Fatalf("inspection changed identity of %s: %v", relative, err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != body {
			t.Fatalf("inspection changed %s: %q %v", relative, data, err)
		}
	}
	if dest, err := os.Readlink(symlink); err != nil || dest != "20.p1.txt.refused" {
		t.Fatalf("inspection changed the unrecognized symlink: %q %v", dest, err)
	}
}
