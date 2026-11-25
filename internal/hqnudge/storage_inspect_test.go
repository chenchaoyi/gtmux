package hqnudge

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSW004WakeStorageInventory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	fixtures := map[string]string{
		"hq-nudges/10.p1.txt":                  "not parsed",
		"hq-nudges/20.p1.txt.sending":          "claimed",
		"hq-nudges/30.p1.txt.native-uncertain": "unknown",
		"hq-nudges/40.p1.txt.legacy-uncertain": "unknown",
		"hq-nudges/foreign-file":               "unclassified",
		"hq-wake-accepted/receipt.json":        "not parsed",
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
	usage, err := InspectStorage()
	if want := (StorageUsage{Queued: 1, Claimed: 1, Uncertain: 2, AwaitingProof: 1, Unclassified: 1}); err != nil || usage != want {
		t.Fatalf("usage %+v, want %+v (%v)", usage, want, err)
	}
	for relative, body := range fixtures {
		data, err := os.ReadFile(state.WorkflowPath(filepath.Dir(relative), filepath.Base(relative)))
		if err != nil || string(data) != body {
			t.Fatalf("inspection changed %s: %q %v", relative, data, err)
		}
	}
}
