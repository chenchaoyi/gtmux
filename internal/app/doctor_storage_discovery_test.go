package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestDoctorStorageMigrationDiscovery(t *testing.T) {
	for _, tc := range []struct {
		name, layout, process, want string
		insideTmux, occupied        bool
	}{
		{name: "ready", layout: "v2", want: "No blocker found in this check"},
		{name: "inside tmux", layout: "v2", insideTmux: true, want: "Migration deferred: the current terminal is inside tmux"},
		{name: "detached tmux", layout: "v2", process: "tmux:", want: "Migration deferred: tmux: is still running (PID 71)"},
		{name: "menu bar", layout: "v2", process: "GtmuxBar", want: "Migration deferred: quit the gtmux menu bar app (PID 71)"},
		{name: "occupied destination", layout: "v2", occupied: true, want: "Migration deferred: destination already exists"},
		{name: "flat", layout: "flat"},
		{name: "already grouped", layout: "v3"},
		{name: "prepared", layout: "prepared", want: "continue or cancel the prepared move"},
		{name: "prepared conflict", layout: "conflict", want: "the prepared service fence does not match the base layout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			bin := t.TempDir()
			t.Setenv("PATH", bin)
			mutations := filepath.Join(bin, "mutations")
			t.Setenv("GTMUX_TEST_STORAGE_MUTATIONS", mutations)
			ps := "#!/bin/sh\nexit 0\n"
			if tc.process != "" {
				ps = fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%d 71 %s'\n", os.Getuid(), tc.process)
			}
			for name, data := range map[string]string{"ps": ps, "launchctl": `#!/bin/sh
case "$1" in
print) printf 'Could not find service "%s" in domain fixture\n' "${2##*/}" >&2; exit 113 ;;
bootout|bootstrap|disable|enable|kickstart) printf '%s\n' "$1" >> "$GTMUX_TEST_STORAGE_MUTATIONS"; exit 2 ;;
*) exit 2 ;;
esac
`, "systemctl": `#!/bin/sh
case "$2" in
show) printf 'LoadState=not-found\nActiveState=inactive\nSubState=dead\nUnitFileState=\nJob=0\n' ;;
start|stop|disable|enable|restart) printf '%s\n' "$2" >> "$GTMUX_TEST_STORAGE_MUTATIONS"; exit 2 ;;
*) exit 2 ;;
esac
`} {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(data), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if tc.layout != "flat" {
				if err := state.ChangeStorageLayout(true, func() error { return nil }); err != nil {
					t.Fatal(err)
				}
			}
			source := filepath.Join(state.Dir(), "hq-nudges", "batch.txt")
			if err := state.WritePrivateFile(source, []byte("preserved")); err != nil {
				t.Fatal(err)
			}
			destination := filepath.Join(state.Dir(), "layout-v3", "hq", "wake", "delivery-queue")
			if tc.occupied {
				if err := os.MkdirAll(destination, 0700); err != nil {
					t.Fatal(err)
				}
			}
			marker := filepath.Join(state.Dir(), ".storage-layout-v3")
			switch tc.layout {
			case "v3":
				if err := state.ChangeWorkflowLayout(true, func() error { return nil }); err != nil {
					t.Fatal(err)
				}
				source = filepath.Join(destination, "batch.txt")
			case "prepared", "conflict":
				if err := state.PrepareWorkflowServiceFence(); err != nil {
					t.Fatal(err)
				}
				if tc.layout == "conflict" {
					if err := os.Mkdir(marker, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			before, err := os.Stat(source)
			if err != nil {
				t.Fatal(err)
			}
			output := runStorageDoctorPTY(t, "n", func() {
				if tc.insideTmux {
					t.Setenv("TMUX", "test-socket,123,0")
				}
				Run([]string{"--lang=en", "doctor"})
			})
			_, summary, found := strings.Cut(output, "Check complete")
			if !found || !strings.Contains(output, tc.want) {
				t.Fatalf("Doctor did not report %q: %s", tc.want, output)
			}
			if tc.layout == "v2" {
				if !strings.Contains(summary, "Optional maintenance") || !strings.Contains(summary, "to layout-v3") || !strings.Contains(summary, tc.want) {
					t.Fatalf("migration advice missing from the summary: %s", summary)
				}
				command := "gtmux doctor --fix"
				if tc.occupied {
					command = "gtmux doctor --storage-layout"
				}
				if !strings.Contains(summary[strings.Index(summary, "Optional maintenance"):], command) {
					t.Fatalf("migration next step missing: %s", summary)
				}
			} else if strings.Contains(output, "Optional maintenance") {
				t.Fatalf("unexpected fresh migration advice: %s", output)
			}
			if strings.Contains(output, "group storage, and restore them?") {
				t.Fatal("ordinary Doctor entered the migration wizard")
			}
			after, err := os.Stat(source)
			if err != nil || !os.SameFile(before, after) {
				t.Fatalf("Doctor moved the original store: %v", err)
			}
			if _, err := os.Stat(mutations); !os.IsNotExist(err) {
				t.Fatalf("unexpected service mutation: %v", err)
			}
			if tc.layout == "v2" {
				status, err := state.ReadStorageLayout()
				if err != nil || status.Workflow != "" || status.Mode != "grouped" {
					t.Fatalf("Doctor changed the layout: %+v, %v", status, err)
				}
			}
		})
	}
}
