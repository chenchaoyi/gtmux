package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateWriteRefusesLinksAndSpecialTargets(t *testing.T) {
	for _, kind := range []string{"root", "parent", "intermediate", "leaf", "directory target", "outside root"} {
		t.Run(kind, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			outside := t.TempDir()
			sentinel := filepath.Join(outside, "sentinel")
			if err := os.WriteFile(sentinel, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(sentinel, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(outside, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(Dir(), "uploads", "nested", "sentinel")
			link := filepath.Join(Dir(), "uploads")
			switch kind {
			case "root":
				link = Dir()
			case "intermediate":
				link = filepath.Join(Dir(), "uploads", "nested")
			case "leaf":
				link = path
			case "directory target":
				link = path
			case "outside root":
				path = sentinel
			}
			if kind != "outside root" {
				if err := os.MkdirAll(filepath.Dir(link), 0700); err != nil {
					t.Fatal(err)
				}
				if kind == "directory target" {
					if err := os.Mkdir(link, 0700); err != nil {
						t.Fatal(err)
					}
				} else {
					target := outside
					if kind == "leaf" {
						target = sentinel
					}
					if err := os.Symlink(target, link); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := WritePrivateFile(path, []byte("replacement")); err == nil {
				t.Fatal("accepted unsafe destination")
			}
			data, err := os.ReadFile(sentinel)
			if err != nil || string(data) != "untouched" || mode(t, sentinel) != 0644 || mode(t, outside) != 0755 {
				t.Fatalf("external target was changed: %q %v", data, err)
			}
			entries, err := os.ReadDir(outside)
			if err != nil || len(entries) != 1 {
				t.Fatalf("wrote outside tree: %v %v", entries, err)
			}
		})
	}
}

func TestPrivateWriteReplacesOnlyOwnedDestination(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, root := range []string{Dir(), ConfigDir()} {
		path := filepath.Join(root, "one", "two", "target")
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		for _, dir := range []string{root, filepath.Join(root, "one"), filepath.Dir(path)} {
			if err := os.Chmod(dir, 0755); err != nil {
				t.Fatal(err)
			}
		}
		outside := filepath.Join(t.TempDir(), "hardlink-source")
		if err := os.WriteFile(outside, []byte("old"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(outside, 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.Link(outside, path); err != nil {
			t.Fatal(err)
		}
		sibling := filepath.Join(filepath.Dir(path), "unrelated")
		if err := os.WriteFile(sibling, []byte("leave me"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(sibling, 0644); err != nil {
			t.Fatal(err)
		}
		if err := WritePrivateFile(path, []byte("new")); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "new" || mode(t, path) != 0600 {
			t.Fatalf("publish: %q %v", data, err)
		}
		data, err = os.ReadFile(outside)
		if err != nil || string(data) != "old" || mode(t, outside) != 0644 || mode(t, sibling) != 0644 {
			t.Fatal("changed unrelated inode")
		}
		for _, dir := range []string{root, filepath.Join(root, "one"), filepath.Dir(path)} {
			if mode(t, dir) != 0700 {
				t.Fatalf("directory not private: %s", dir)
			}
		}
		entries, err := os.ReadDir(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".private-") {
				t.Fatal("temporary file leaked")
			}
		}
	}
}

// Concurrent readers may see the old version or the complete replacement, never
// truncated bytes. Inspect staging files too: rename preserves their mode.
func TestPrivateWritePublishesCompletePrivateBytes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	path := filepath.Join(Dir(), "uploads", "image")
	a, b := strings.Repeat("a", 1<<20), strings.Repeat("b", 1<<20)
	if err := WritePrivateFile(path, []byte(a)); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				done <- nil
				return
			default:
			}
			data, err := os.ReadFile(path)
			if err != nil {
				done <- err
				return
			}
			if string(data) != a && string(data) != b {
				done <- fmt.Errorf("reader saw partial data: %d bytes", len(data))
				return
			}
			entries, err := os.ReadDir(filepath.Dir(path))
			if err != nil {
				done <- err
				return
			}
			for _, e := range entries {
				fi, err := e.Info()
				if os.IsNotExist(err) {
					continue
				} // renamed/removed between observations
				if err != nil {
					done <- err
					return
				}
				if fi.Mode().Perm()&0077 != 0 {
					done <- fmt.Errorf("exposed staging/published mode: %o", fi.Mode().Perm())
					return
				}
			}
		}
	}()
	var writeErr error
	for i := 0; i < 20; i++ {
		data := a
		if i%2 == 0 {
			data = b
		}
		if writeErr = WritePrivateFile(path, []byte(data)); writeErr != nil {
			break
		}
	}
	close(stop)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}
