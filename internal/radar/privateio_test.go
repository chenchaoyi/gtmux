package radar

import (
	"bytes"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/assets"
)

func TestIconWriterIsLocallyPrivateIncludingCacheHits(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	path := builtinIconCachePath("codex")
	for _, stale := range []string{"new", "identical", "corrupt-temp"} {
		if stale != "new" {
			if err := os.Chmod(path, 0644); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(filepath.Dir(path), 0755); err != nil {
				t.Fatal(err)
			}
			if stale == "corrupt-temp" {
				if err := os.WriteFile(path, []byte("old"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path+".tmp", []byte("stale"), 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		got := BuiltinIconPath("codex")
		if got != path {
			t.Fatalf("%s returned %q", stale, got)
		}
		data, err := os.ReadFile(got)
		if err != nil || !bytes.Equal(data, assets.AgentIcon("codex")) {
			t.Fatal("icon content changed", err)
		}
		for p, want := range map[string]os.FileMode{got: 0600, filepath.Dir(got): 0700} {
			info, err := os.Stat(p)
			if err != nil || info.Mode().Perm() != want {
				t.Errorf("%s %s: %v %v", stale, p, info, err)
			}
		}
	}
}
