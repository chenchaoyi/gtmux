package app

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/creack/pty"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/userservice"
)

type storageLaunchdFixture struct {
	t             *testing.T
	manager       userservice.StorageLaunchd
	overrides     map[string]bool
	loaded        bool
	foreignPath   bool
	failBootstrap bool
	mutations     []string
}

func newStorageLaunchdFixture(t *testing.T) *storageLaunchdFixture {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	if err := state.ChangeStorageLayout(true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	if err := state.WritePrivateFile(filepath.Join(state.Dir(), "hq-nudges", "batch.txt"), []byte("keep this batch")); err != nil {
		t.Fatal(err)
	}
	launchd := userservice.Launchd{Home: home}
	if err := os.MkdirAll(launchd.Dir(), 0700); err != nil {
		t.Fatal(err)
	}
	fixture := &storageLaunchdFixture{t: t, loaded: true, overrides: map[string]bool{}}
	labels := []string{serveAgentLabel, tunnelAgentLabel, selfTunnelAgentLabel}
	missing := exec.Command("sh", "-c", "exit 113").Run()
	uid := strconv.Itoa(os.Getuid())
	for _, label := range []string{serveAgentLabel, selfTunnelAgentLabel} {
		if err := os.WriteFile(launchd.Path(label), []byte(label), 0600); err != nil {
			t.Fatal(err)
		}
		for _, domain := range []string{"gui/" + uid, "user/" + uid} {
			fixture.overrides[domain+"/"+label] = label != serveAgentLabel
		}
	}
	fixture.manager = userservice.StorageLaunchd{
		Home: home, Labels: labels,
		Probe: func(name string, args ...string) ([]byte, error) {
			t.Helper()
			if name != "launchctl" || len(args) < 2 {
				t.Fatalf("unexpected process: %s %q", name, args)
			}
			switch args[0] {
			case "print":
				if args[1] == "gui/"+uid+"/"+serveAgentLabel && fixture.loaded {
					path := launchd.Path(serveAgentLabel)
					if fixture.foreignPath {
						path = filepath.Join(home, "another.plist")
					}
					return fmt.Appendf(nil, "\tpath = %s\n\tpid = 41\n", path), nil
				}
				return fmt.Appendf(nil, "Could not find service %q in domain for uid: %s", filepath.Base(args[1]), uid), missing
			case "print-disabled":
				var rows strings.Builder
				rows.WriteString("disabled services = {\n")
				for _, label := range labels {
					if disabled, ok := fixture.overrides[args[1]+"/"+label]; ok {
						fmt.Fprintf(&rows, "%q => %t\n", label, disabled)
					}
				}
				rows.WriteString("}\n")
				return []byte(rows.String()), nil
			}
			if jobs, err := readStorageServicePlan(); err != nil || len(jobs) != 2 || !jobs[0].Loaded || jobs[1].Loaded {
				t.Fatalf("pre-mutation service plan: %+v, %v", jobs, err)
			}
			fixture.mutations = append(fixture.mutations, args[0])
			switch args[0] {
			case "disable", "enable":
				if filepath.Base(args[1]) != serveAgentLabel {
					t.Fatalf("changed an originally disabled job: %q", args)
				}
				fixture.overrides[args[1]] = args[0] == "disable"
			case "bootout":
				fixture.loaded = false
			case "bootstrap":
				if fixture.failBootstrap {
					return nil, errors.New("fixture bootstrap failed")
				}
				if status, err := state.ReadStorageLayout(); err != nil || !status.WorkflowActive() {
					t.Fatalf("bootstrap before layout readback: %+v, %v", status, err)
				}
				fixture.loaded = true
			default:
				t.Fatalf("unexpected launchctl call: %q", args)
			}
			return nil, nil
		},
	}
	return fixture
}

func (f *storageLaunchdFixture) offline() error {
	f.t.Helper()
	if f.loaded {
		f.t.Fatal("layout check ran while the managed job was loaded")
	}
	for _, disabled := range f.overrides {
		if !disabled {
			f.t.Fatal("layout check ran while a managed job could start")
		}
	}
	return nil
}

func (f *storageLaunchdFixture) assertRestored() {
	f.t.Helper()
	jobs, err := f.manager.Inspect()
	if err != nil || len(jobs) != 2 || !jobs[0].Running || jobs[0].Disabled != [2]bool{} ||
		jobs[1].Loaded || jobs[1].Disabled != [2]bool{true, true} {
		f.t.Fatalf("original jobs not restored: %+v, %v", jobs, err)
	}
	if plan, err := readStorageServicePlan(); err != nil || plan != nil {
		f.t.Fatalf("service plan was not retired: %+v, %v", plan, err)
	}
}

func TestDoctorGuidedStorageRestoresManagedServices(t *testing.T) {
	fixture := newStorageLaunchdFixture(t)
	processes := func(pids []int) error {
		if !slices.Equal(pids, []int{41}) {
			t.Fatalf("allowed process identities: %v", pids)
		}
		return nil
	}
	confirms := 0
	confirm := func(prompt string) bool {
		confirms++
		if !strings.Contains(prompt, "tmux") || len(fixture.mutations) != 0 {
			t.Fatalf("unexpected confirmation or early launchd changes: %s, %v", prompt, fixture.mutations)
		}
		return true
	}
	if code := guidedDoctorStorageChange(fixture.manager, processes, fixture.offline, confirm); code != 0 || confirms != 1 {
		t.Fatalf("Doctor returned %d with %d confirmations", code, confirms)
	}
	want := []string{"disable", "disable", "bootout", "enable", "enable", "bootstrap"}
	if !slices.Equal(fixture.mutations, want) {
		t.Fatalf("launchd mutations %v, want %v", fixture.mutations, want)
	}
	if got, err := os.ReadFile(state.WorkflowPath("hq-nudges", "batch.txt")); err != nil || string(got) != "keep this batch" {
		t.Fatalf("migrated contents %q: %v", got, err)
	}
	fixture.assertRestored()
}

func TestDoctorGuidedStorageFixEntry(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS Doctor entry")
	}
	t.Setenv("TMUX", "")
	t.Setenv("HOME", t.TempDir())
	if err := state.ChangeStorageLayout(true, func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	fakeCommands := map[string]string{
		"ps": "#!/bin/sh\nexit 0\n",
		"launchctl": `#!/bin/sh
case "$1" in
print) printf 'Could not find service "%s" in domain fixture\n' "${2##*/}" >&2; exit 113 ;;
*) exit 2 ;;
esac
`,
	}
	for name, script := range fakeCommands {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(script), 0700); err != nil {
			t.Fatal(err)
		}
	}
	master, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer tty.Close()
	originalStdin := os.Stdin
	os.Stdin = tty
	defer func() { os.Stdin = originalStdin }()
	if _, err := master.Write([]byte("y\n")); err != nil {
		t.Fatal(err)
	}
	output := captureStdout(t, func() {
		if rc := Run([]string{"--lang=en", "doctor", "--fix"}); rc != 0 {
			t.Fatalf("doctor --fix returned %d", rc)
		}
	})
	if !strings.Contains(output, "temporarily pause the managed gtmux services") || strings.Count(output, "[y/N]") != 1 {
		t.Fatalf("doctor --fix did not use its single migration prompt: %s", output)
	}
	if status, err := state.ReadStorageLayout(); err != nil || !status.WorkflowActive() {
		t.Fatalf("doctor --fix layout: %+v, %v", status, err)
	}
}

func TestDoctorGuidedStorageBlocksWithoutTouchingServices(t *testing.T) {
	for _, test := range []struct {
		name       string
		blockCheck int
		blocker    string
		foreign    bool
		accept     bool
		want       int
	}{
		{name: "existing tmux", blockCheck: 1, blocker: "tmux PID 71", want: 1},
		{name: "menu bar", blockCheck: 1, blocker: "gtmux menu bar PID 72", want: 1},
		{name: "tmux appears after confirmation", blockCheck: 2, blocker: "tmux PID 73", accept: true, want: 1},
		{name: "unknown job source", foreign: true, want: 1},
		{name: "declined confirmation"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newStorageLaunchdFixture(t)
			fixture.foreignPath = test.foreign
			checks := 0
			processes := func([]int) error {
				checks++
				if checks == test.blockCheck {
					return errors.New(test.blocker)
				}
				return nil
			}
			code := guidedDoctorStorageChange(fixture.manager, processes, fixture.offline, func(string) bool { return test.accept })
			if code != test.want || len(fixture.mutations) != 0 {
				t.Fatalf("Doctor returned %d, launchd mutations: %v", code, fixture.mutations)
			}
			status, err := state.ReadStorageLayout()
			if err != nil || status.Workflow != "" {
				t.Fatalf("blocked change still altered layout: %+v, %v", status, err)
			}
			if !test.foreign {
				fixture.assertRestored()
			}
		})
	}
}

func TestDoctorGuidedStorageContinuesAfterInterruption(t *testing.T) {
	for _, failure := range []string{"late writer", "bootstrap failure"} {
		t.Run(failure, func(t *testing.T) {
			fixture := newStorageLaunchdFixture(t)
			fixture.failBootstrap = failure == "bootstrap failure"
			processes := func([]int) error { return nil }
			confirms := 0
			confirm := func(string) bool { confirms++; return true }
			checks := 0
			writers := func() error {
				checks++
				if failure == "late writer" && checks == 2 {
					return errors.New("fixture process appeared after journal publication")
				}
				return fixture.offline()
			}
			if code := guidedDoctorStorageChange(fixture.manager, processes, writers, confirm); code != 1 || fixture.loaded {
				t.Fatalf("failure returned %d, service loaded: %v", code, fixture.loaded)
			}
			if jobs, err := readStorageServicePlan(); err != nil || len(jobs) != 2 || !jobs[0].Loaded {
				t.Fatalf("missing original service plan after failure: %+v, %v", jobs, err)
			}
			wantConfirms := 1
			if failure == "late writer" {
				if err := state.CheckStorageLayout(); err == nil || !slices.Equal(fixture.mutations, []string{"disable", "disable", "bootout"}) {
					t.Fatalf("writers resumed before layout recovery: %v, %v", err, fixture.mutations)
				}
				wantConfirms = 2
			}
			fixture.failBootstrap = false
			if code := guidedDoctorStorageChange(fixture.manager, processes, fixture.offline, confirm); code != 0 || confirms != wantConfirms {
				t.Fatalf("continuation returned %d with %d confirmations, want %d", code, confirms, wantConfirms)
			}
			fixture.assertRestored()
		})
	}
}
