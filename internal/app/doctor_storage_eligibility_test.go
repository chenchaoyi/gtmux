package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestDoctorStorageMigrationEligibility(t *testing.T) {
	for _, tc := range []struct {
		name, want string
	}{
		{name: "new installation", want: "flat"},
		{name: "empty v2", want: "layout-v2"},
		{name: "empty scaffolding and locks", want: "layout-v2"},
		{name: "completed v3", want: "layout-v3"},
		{name: "unverified marker", want: "workflow layout marker exists without a verified journal"},
		{name: "v3 with an inactive path", want: "a writer recreated an inactive workflow path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("PATH", t.TempDir())
			stopped := func() error { return nil }
			if tc.name != "new installation" {
				if err := state.ChangeStorageLayout(true, stopped); err != nil {
					t.Fatal(err)
				}
			}
			root := state.Dir()
			marker := filepath.Join(root, ".storage-layout-v3")
			switch tc.name {
			case "empty scaffolding and locks":
				if err := os.MkdirAll(filepath.Join(root, "hq-nudges", "empty", "nested"), 0700); err != nil {
					t.Fatal(err)
				}
				for _, name := range []string{"lease-notify-queue.lock", "hqwake/lease-wake-observations.lock", "hqwake/accepted-archive.jsonl"} {
					if err := state.WritePrivateFile(filepath.Join(root, name), nil); err != nil {
						t.Fatal(err)
					}
				}
			case "unverified marker":
				if err := os.Mkdir(marker, 0700); err != nil {
					t.Fatal(err)
				}
			case "completed v3", "v3 with an inactive path":
				if err := state.ChangeWorkflowLayout(true, stopped); err != nil {
					t.Fatal(err)
				}
				if tc.name == "v3 with an inactive path" {
					if err := state.WritePrivateFile(filepath.Join(root, "hq-nudges", "old.txt"), []byte("review me")); err != nil {
						t.Fatal(err)
					}
				}
			}
			before, markerErr := os.Stat(marker)
			if markerErr != nil && !os.IsNotExist(markerErr) {
				t.Fatal(markerErr)
			}
			output := runStorageDoctorPTY(t, "n", func() { Run([]string{"--lang=en", "doctor"}) })
			if strings.Contains(output, "Optional maintenance") || !strings.Contains(output, tc.want) {
				t.Fatalf("wanted diagnostic %q without a migration offer: %s", tc.want, output)
			}
			after, err := os.Stat(marker)
			if before != nil {
				if err != nil || !os.SameFile(before, after) {
					t.Fatalf("Doctor changed workflow marker: %v", err)
				}
			} else if !os.IsNotExist(err) {
				t.Fatalf("Doctor created a workflow marker: %v", err)
			}
		})
	}
}
