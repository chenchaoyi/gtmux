package app

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Exercise the actual byte-copy and HTTP callback, including callers that do not
// go through main's umask. A source file's public mode must never be copied.
func TestUploadWritersKeepPrivateModes(t *testing.T) {
	for _, mask := range []int{0, 0022, 0077} {
		t.Run(fmt.Sprintf("umask-%03o", mask), func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			old := syscall.Umask(mask)
			defer syscall.Umask(old)
			src := filepath.Join(t.TempDir(), "screenshot.png")
			if err := os.WriteFile(src, []byte("image"), 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(src, 0644); err != nil {
				t.Fatal(err)
			}
			paths, err := attachFiles([]string{src})
			if err != nil {
				t.Fatal(err)
			}
			check := func(path string, want os.FileMode) {
				t.Helper()
				fi, err := os.Lstat(path)
				if err != nil {
					t.Fatal(err)
				}
				if fi.Mode().Perm() != want {
					t.Errorf("%s: mode %04o, want %04o", filepath.Base(path), fi.Mode().Perm(), want)
				}
			}
			check(paths[0], 0600)
			check(state.Dir(), 0700)
			check(uploadsDir(), 0700)
			// Simulate an older copy that is identical, then a corrupt existing target.
			// Publishing again must narrow only this destination, without touching links.
			for _, contents := range []string{"image", "corrupt"} {
				if err := os.WriteFile(paths[0], []byte(contents), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(paths[0], 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(uploadsDir(), 0755); err != nil {
					t.Fatal(err)
				}
				again, err := attachFiles([]string{src})
				if err != nil {
					t.Fatal(err)
				}
				if again[0] != paths[0] {
					t.Fatal("retry path changed")
				}
				check(again[0], 0600)
				check(uploadsDir(), 0700)
				if data, err := os.ReadFile(again[0]); err != nil || string(data) != "image" {
					t.Fatalf("data=%q err=%v", data, err)
				}
			}
			uploaded, err := saveUpload("screenshot.png", []byte("phone"))
			if err != nil {
				t.Fatal(err)
			}
			check(uploaded, 0600)
			check(src, 0644)
			entries, err := os.ReadDir(uploadsDir())
			if err != nil || len(entries) != 2 {
				t.Fatalf("temporary files left behind: %v %v", entries, err)
			}
		})
	}
}
