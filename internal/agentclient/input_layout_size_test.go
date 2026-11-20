package agentclient

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSPL010LayoutReadMatchesOpenFileSize(t *testing.T) {
	const valid = `{"version":1}`
	for _, tc := range []struct {
		name string
		size int64
		ok   bool
	}{
		{"exact", int64(len(valid)), true},
		{"valid JSON grew", int64(len(valid)) - 1, false},
		{"valid JSON shrank", int64(len(valid)) + 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := readInputSnapshot(strings.NewReader(valid), tc.size)
			if tc.ok {
				if err != nil || string(b) != valid {
					t.Fatalf("matching snapshot refused: %q, %v", b, err)
				}
			} else if !errors.Is(err, errInvalidInputJSON) {
				t.Fatalf("changed size accepted: %q, %v", b, err)
			}
		})
	}
	for _, doc := range []string{inputStoreJournal, legacyInputDir} {
		t.Run(doc, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "state")
			if err := os.MkdirAll(filepath.Join(root, legacyInputDir), 0700); err != nil {
				t.Fatal(err)
			}
			stageGateFixture(t, root)("active")
			path := filepath.Join(root, doc)
			json, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			atLimit := string(json) + strings.Repeat(" ", inputStoreMetadataLimit-len(json))
			for _, size := range []int{inputStoreMetadataLimit, inputStoreMetadataLimit + 1} {
				if err := os.WriteFile(path, []byte(atLimit+strings.Repeat(" ", size-len(atLimit))), 0600); err != nil {
					t.Fatal(err)
				}
				store, err := resolveInputStore(root)
				if size == inputStoreMetadataLimit {
					if err != nil || store.token != gateToken {
						t.Fatalf("exact byte limit refused: %+v, %v", store, err)
					}
				} else if !errors.Is(err, ErrInputStoreUnavailable) || store.path != "" {
					t.Fatalf("oversized document selected a path: %+v, %v", store, err)
				}
			}
		})
	}
}
