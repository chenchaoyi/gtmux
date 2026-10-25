package usage

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestDailyWriterIsLocallyPrivate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	for _, stale := range []bool{false, true} {
		if stale {
			for _, p := range []string{dailyPath(), dailyPath() + ".tmp", dailyPath() + ".lock"} {
				if err := os.WriteFile(p, []byte("stale"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(p, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		saveDaily(dailyLedger{V: 1, Files: map[string]fileMark{}, Days: map[string]map[string]*Count{}})
		info, err := os.Stat(dailyPath())
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0600 {
			t.Errorf("daily stale=%v: mode=%o", stale, info.Mode().Perm())
		}
		if loadDaily().V != 1 {
			t.Fatal("daily format changed")
		}
		for _, p := range []string{state.Dir(), filepath.Dir(dailyPath())} {
			info, err := os.Stat(p)
			if err != nil || info.Mode().Perm() != 0700 {
				t.Errorf("directory %s: %v %v", p, info, err)
			}
		}
	}
}
