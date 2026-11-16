package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/hq"
	"github.com/chenchaoyi/gtmux/internal/i18n"
)

func TestDPC006DoctorExplainsActualCountAndByteFailures(t *testing.T) {
	old := i18n.Lang()
	t.Cleanup(func() { i18n.SetLang(old) })
	for _, cause := range []string{"file-count", "bytes"} {
		t.Run(cause, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("XDG_DATA_HOME", t.TempDir())
			root := hq.MemoryRoot()
			if err := os.MkdirAll(filepath.Join(root, "notes"), 0700); err != nil {
				t.Fatal(err)
			}
			count, size := 10001, int64(0)
			if cause == "bytes" {
				count, size = 4, 32<<20
			}
			for i := 0; i < count; i++ {
				name := filepath.Join(root, "notes", fmt.Sprintf("ordinary-%05d.txt", i))
				f, err := os.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					t.Fatal(err)
				}
				err = f.Truncate(size) // Sparse fixtures exercise metadata limits, not disk allocation.
				closeErr := f.Close()
				if err != nil || closeErr != nil {
					t.Fatal(err, closeErr)
				}
			}
			dst := filepath.Join(t.TempDir(), "existing.tar.gz")
			if err := os.WriteFile(dst, []byte("existing recovery point"), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := hq.ExportMemory(dst)
			if err == nil || !strings.HasPrefix(err.Error(), "HQ records exceed restore limits") {
				t.Fatalf("did not observe actual %s preflight failure: %v", cause, err)
			}
			for _, language := range []string{"en", "zh"} {
				i18n.SetLang(language)
				note := doctorSnapshotFailure(err.Error())
				if !strings.Contains(note, "128 MiB") || !strings.Contains(note, "10,000") ||
					!strings.Contains(note, i18n.Tr("or", "或")) || strings.Contains(note, "64 MiB") {
					t.Fatal("actual failure was misrepresented", note)
				}
			}
			if data, err := os.ReadFile(dst); err != nil || string(data) != "existing recovery point" {
				t.Fatal("failed export replaced a recovery point")
			}
			files, err := os.ReadDir(filepath.Join(root, "notes"))
			if err != nil || len(files) != count {
				t.Fatal("failed export changed protected source entries")
			}
			for _, file := range files {
				if info, err := file.Info(); err != nil || info.Size() != size {
					t.Fatal("failed export changed protected source length")
				}
			}
		})
	}
}
