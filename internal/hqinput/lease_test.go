package hqinput

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompetingChannelsHoldRatherThanOverwriteAndResumeAfterRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hq-input.lock")
	release, ok, err := acquire(path)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	if other, ok, err := acquire(path); err != nil || ok || other != nil {
		t.Fatal("competing writer acquired", ok, err)
	}
	release()
	next, ok, err := acquire(path)
	if err != nil || !ok {
		t.Fatal("lease did not release", ok, err)
	}
	next()
	st, _ := os.Stat(path)
	if st.Mode().Perm() != 0600 {
		t.Fatal("input lock is not private")
	}
}
func TestInputLeaseFailureIsExplicit(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "file")
	_ = os.WriteFile(file, []byte("not a directory"), 0600)
	if release, ok, err := acquire(filepath.Join(file, "lock")); err == nil || ok || release != nil {
		t.Fatal("lock IO failure hidden", ok, err)
	}
}
