package state

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestGSS019RollbackSyncsMarkerBeforeScaffold(t *testing.T) {
	for _, mode := range []string{"empty scaffold", "staging archive"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			origin := layoutFixture(t, "events.seq", "19")
			if err := ChangeStorageLayout(true, layoutWritersStopped); err != nil {
				t.Fatal(err)
			}
			var staged os.FileInfo
			var stagedRel string
			if mode == "staging archive" {
				stagedRel = filepath.Join("usage", privateTempPrefix+rand.Text())
				staged = layoutFixture(t, filepath.Join(layoutDir, stagedRel), "scratch")
			}
			tx, journal := pauseLayoutForCommitTest(t, false)
			failedSync := errors.New("synthetic sync failure")
			for attempt := 0; attempt < 2; attempt++ {
				calls := 0
				err := finishLayoutChangeWithSync(tx, func(path string) error {
					calls++
					if path != Dir() {
						t.Fatalf("wrong parent sync: %s", path)
					}
					if _, present, err := readLayoutMarker(); err != nil || present {
						t.Fatalf("rollback marker was not removed before sync: %v", err)
					}
					return failedSync
				})
				if !errors.Is(err, failedSync) || calls != 1 {
					t.Fatalf("attempt %d passed the required directory sync: %v, calls %d", attempt, err, calls)
				}
				if err := tx.Scaffold.verify(groupedRoot(), true); err != nil {
					t.Fatalf("scaffold changed before marker sync: %v", err)
				}
				if staged != nil {
					wantLayoutContents(t, filepath.Join(groupedRoot(), stagedRel), "scratch", staged)
				}
				wantLayoutContents(t, filepath.Join(Dir(), layoutTransition), string(journal), nil)
				wantLayoutContents(t, filepath.Join(Dir(), "events.seq"), "19", origin)
				tx, err = readLayoutTransaction()
				if err != nil || tx == nil {
					t.Fatalf("could not read the original journal after sync failure: %v", err)
				}
			}
			if err := ChangeStorageLayout(false, layoutWritersStopped); err != nil {
				t.Fatalf("rollback did not recover after directory sync worked: %v", err)
			}
			if err := CheckStorageLayout(); err != nil {
				t.Fatal(err)
			}
			if staged != nil {
				wantLayoutContents(t, filepath.Join(tx.retainedPath(), stagedRel), "scratch", staged)
			}
			wantLayoutContents(t, StorePath("events.seq"), "19", origin)
		})
	}
}
