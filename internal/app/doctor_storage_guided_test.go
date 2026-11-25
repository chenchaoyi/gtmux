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
	t              *testing.T
	manager        userservice.StorageLaunchd
	overrides      map[string]bool
	loaded         bool
	directLoaded   bool
	idle           bool
	foreignPath    bool
	foreignProgram bool
	legacyMenu     bool
	failBootstrap  bool
	mutations      []string
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
	program := filepath.Join(home, ".local", "bin", "gtmux")
	argv := map[string][]string{
		serveAgentLabel:      {program, "serve", "--bind", "127.0.0.1", "--port", "8765"},
		selfTunnelAgentLabel: {program, "tunnel-client", "--port", "8765"},
	}
	for _, label := range []string{serveAgentLabel, selfTunnelAgentLabel} {
		if err := launchd.Write(launchd.Path(label), label, argv[label], filepath.Join(state.Dir(), label+".log")); err != nil {
			t.Fatal(err)
		}
		if label == selfTunnelAgentLabel {
			fixture.overrides["gui/"+uid+"/"+label] = true
			fixture.overrides["user/"+uid+"/"+label] = true
		}
	}
	fixture.manager = userservice.StorageLaunchd{
		Home: home, Labels: labels,
		Probe: func(name string, args ...string) ([]byte, error) {
			t.Helper()
			if name != "launchctl" || len(args) < 2 {
				t.Fatalf("unexpected process: %s %q", name, args)
			}
			label := filepath.Base(args[len(args)-1])
			label = strings.TrimSuffix(label, ".plist")
			switch args[0] {
			case "print":
				if label == menubarBundleID && fixture.legacyMenu && strings.HasPrefix(args[1], "gui/") {
					return []byte("loaded legacy fixture"), nil
				}
				active := label == serveAgentLabel && fixture.loaded || label == selfTunnelAgentLabel && fixture.directLoaded
				if strings.HasPrefix(args[1], "gui/") && active {
					path, exe := launchd.Path(label), program
					if fixture.foreignPath {
						path = filepath.Join(home, "another.plist")
					}
					if fixture.foreignProgram {
						exe = "/usr/bin/true"
					}
					body := fmt.Sprintf("\tpath = %s\n\tprogram = %s\n\targuments = {\n\t\t%s\n\t}\n", path, exe, strings.Join(argv[label], "\n\t\t"))
					if label != serveAgentLabel || !fixture.idle {
						body += "\tpid = 41\n"
					}
					return []byte(body), nil
				}
				return fmt.Appendf(nil, "Could not find service %q in domain for uid: %s", label, uid), missing
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
			if plan, err := readStorageServicePlan(); err != nil || plan == nil || len(plan.Jobs) != 2 || !plan.Jobs[0].Loaded {
				t.Fatalf("pre-mutation service plan: %+v, %v", plan, err)
			}
			fixture.mutations = append(fixture.mutations, args[0]+":"+label)
			switch args[0] {
			case "bootout":
				if _, parked, _, err := state.FindStorageServiceProfile(label); err != nil || !parked {
					t.Fatalf("job stopped before its profile was parked: %v", err)
				}
				if label == serveAgentLabel {
					fixture.loaded = false
				} else {
					fixture.directLoaded = false
				}
			case "bootstrap":
				if fixture.failBootstrap && label == serveAgentLabel {
					return nil, errors.New("fixture bootstrap failed")
				}
				if status, err := state.ReadStorageLayout(); err != nil || !status.WorkflowActive() {
					t.Fatalf("bootstrap before layout readback: %+v, %v", status, err)
				}
				if label == serveAgentLabel {
					fixture.loaded = true
				} else {
					fixture.directLoaded = true
				}
			case "kickstart":
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
	if f.loaded || f.directLoaded {
		f.t.Fatal("layout check ran while a managed job was loaded")
	}
	for _, label := range []string{serveAgentLabel, selfTunnelAgentLabel} {
		if _, parked, _, err := state.FindStorageServiceProfile(label); err != nil || !parked {
			f.t.Fatalf("layout check ran before %s was durably parked: %v", label, err)
		}
	}
	return nil
}

func (f *storageLaunchdFixture) assertRestored() {
	f.t.Helper()
	jobs, err := f.manager.Inspect()
	if err != nil || len(jobs) != 2 || !jobs[0].Running || jobs[0].Override != [2]string{"default", "default"} ||
		jobs[0].Parked || jobs[1].Parked || jobs[1].Loaded || jobs[1].Override != [2]string{"disabled", "disabled"} {
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
	want := []string{"bootout:" + serveAgentLabel, "bootstrap:" + serveAgentLabel}
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
			if plan, err := readStorageServicePlan(); err != nil || plan == nil || len(plan.Jobs) != 2 || !plan.Jobs[0].Loaded {
				t.Fatalf("missing original service plan after failure: %+v, %v", plan, err)
			}
			wantConfirms := 1
			if failure == "late writer" {
				if err := state.CheckStorageLayout(); err == nil || !slices.Equal(fixture.mutations, []string{"bootout:" + serveAgentLabel}) {
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
