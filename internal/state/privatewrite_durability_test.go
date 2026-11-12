package state

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"
)

func TestPrivateWriteRetriesDirectorySyncAfterRename(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(ConfigDir(), "hq", "LOCAL.md")
	if err := WritePrivateFile(path, []byte("manual old bytes")); err != nil {
		t.Fatal(err)
	}
	old, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	data := []byte("manual old bytes\ncomplete generated section\n")
	calls := 0
	syncDirectory := func(fd int) error {
		calls++
		var st unix.Stat_t
		if err := unix.Fstat(fd, &st); err != nil {
			return err
		}
		if st.Mode&unix.S_IFMT != unix.S_IFDIR {
			t.Error("sync target is not the held parent directory")
		}
		visible, err := os.ReadFile(path)
		if err != nil || string(visible) != string(data) {
			t.Error("directory sync ran before complete rename")
		}
		if calls <= 2 {
			return unix.EIO
		}
		return unix.Fsync(fd)
	}
	for attempt := 1; attempt <= 3; attempt++ {
		err := writePrivateFileWithSync(path, data, unix.Renameat, syncDirectory)
		if attempt <= 2 && !errors.Is(err, unix.EIO) {
			t.Fatalf("post-rename error swallowed: %v", err)
		}
		if attempt == 3 && err != nil {
			t.Fatal(err)
		}
		if calls != attempt {
			t.Fatalf("same bytes skipped directory sync: %d", calls)
		}
		if mode(t, path) != 0600 {
			t.Fatal("published permissions are not private")
		}
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".private-") {
				t.Fatal("staging file leaked")
			}
		}
	}
	pinned, err := io.ReadAll(old)
	if err != nil || string(pinned) != "manual old bytes" {
		t.Fatal("old inode was truncated")
	}
}
