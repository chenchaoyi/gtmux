package app

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/creack/pty"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/userservice"
)

func runStorageDoctorPTY(t *testing.T, answer string, run func()) string {
	t.Helper()
	t.Setenv("TMUX", "")
	master, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer master.Close()
	defer tty.Close()
	stdin := os.Stdin
	os.Stdin = tty
	defer func() { os.Stdin = stdin }()
	if _, err := master.Write([]byte(answer + "\n")); err != nil {
		t.Fatal(err)
	}
	return captureStdout(t, run)
}

func fakeRestartedLaunchd(t *testing.T, running string) {
	t.Helper()
	bin := t.TempDir()
	t.Setenv("PATH", bin)
	t.Setenv("GTMUX_TEST_RUNNING_JOB", running)
	t.Setenv("GTMUX_TEST_SERVE_PROFILE", (userservice.Launchd{Home: state.Home()}).Path(serveAgentLabel))
	t.Setenv("GTMUX_TEST_SERVE_BIN", filepath.Join(state.Home(), ".local", "bin", "gtmux"))
	for name, contents := range map[string]string{
		"ps": "#!/bin/sh\nexit 0\n",
		"launchctl": `#!/bin/sh
case "$1" in
print)
  if [ "${2%%/*}" = gui ] && [ "${2##*/}" = com.gtmux.serve ] && [ -f "$GTMUX_TEST_RUNNING_JOB" ]; then
    printf '\tpath = %s\n\tprogram = %s\n\tpid = 41\n\targuments = {\n\t\t%s\n\t\tserve\n\t\t--bind\n\t\t127.0.0.1\n\t\t--port\n\t\t8765\n\t}\n' "$GTMUX_TEST_SERVE_PROFILE" "$GTMUX_TEST_SERVE_BIN" "$GTMUX_TEST_SERVE_BIN"
  else
    printf 'Could not find service "%s" in domain fixture\n' "${2##*/}" >&2
    exit 113
  fi ;;
print-disabled)
  printf 'disabled services = {\n"com.gtmux.selftunnel" => disabled\n}\n' ;;
bootstrap)
  [ "$3" = "$GTMUX_TEST_SERVE_PROFILE" ] && [ -f "$3" ] && [ ! -e "$GTMUX_TEST_RUNNING_JOB" ] || exit 2
  : > "$GTMUX_TEST_RUNNING_JOB" ;;
list)
  [ "$#" = 1 ] ;;
*) exit 2 ;;
esac
`,
	} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte(contents), 0700); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDoctorStorageRebootAfterParkingThroughCLI(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS Doctor entry")
	}
	for _, decision := range []string{"y", "n"} {
		t.Run(decision, func(t *testing.T) {
			fixture := newStorageLaunchdFixture(t)
			jobs, err := fixture.manager.Inspect()
			if err != nil || len(jobs) != 2 || !jobs[0].Loaded {
				t.Fatal(jobs, err)
			}
			plan := &storageServicePlan{Phase: "pause", Jobs: jobs}
			ops := systemStorageServiceIO()
			if err := ops.save(plan); err != nil {
				t.Fatal(err)
			}
			save := func() error { return ops.save(plan) }
			if err := fixture.manager.Stop(plan.Jobs, save); err != nil {
				t.Fatal(err)
			}
			if err := state.PrepareWorkflowServiceFence(); err != nil {
				t.Fatal(err)
			}
			// Crash after moving the profile but before publishing the
			// completed parking stage. Launchd starts again without this file.
			plan.Jobs[0].Stage = "parking"
			if err := ops.save(plan); err != nil {
				t.Fatal(err)
			}
			if err := state.MoveStorageServiceProfile(serveAgentLabel, jobs[0].Identity, true); err != nil {
				t.Fatal(err)
			}
			running := filepath.Join(t.TempDir(), "job-running")
			fakeRestartedLaunchd(t, running)
			output := runStorageDoctorPTY(t, decision, func() {
				if rc := Run([]string{"--lang=en", "doctor", "--fix"}); rc != 0 {
					t.Fatalf("Doctor returned %d", rc)
				}
			})
			if !strings.Contains(output, "[y/N]") {
				t.Fatalf("resume prompt missing: %s", output)
			}
			status, err := state.ReadStorageLayout()
			if err != nil || status.WorkflowActive() != (decision == "y") || state.CheckStorageLayout() != nil {
				t.Fatalf("wrong final layout: %+v, %v", status, err)
			}
			path, parked, identity, err := state.FindStorageServiceProfile(serveAgentLabel)
			if err != nil || path == "" || parked || identity != jobs[0].Identity {
				t.Fatalf("original profile was not returned: %s %v %v", path, parked, err)
			}
			if _, err := os.Stat(running); err != nil {
				t.Fatalf("the recorded loaded service was not restarted: %v", err)
			}
			if plan, err := readStorageServicePlan(); err != nil || plan != nil {
				t.Fatalf("unfinished service plan: %+v %v", plan, err)
			}
		})
	}
}

func TestDoctorStorageFencesLegacyRemoteOff(t *testing.T) {
	oldCLI := os.Getenv("GTMUX_TEST_PRE_MAINTENANCE_CLI")
	for _, window := range []string{"already admitted", "prepared", "moving", "parked", "profile returned"} {
		t.Run(window, func(t *testing.T) {
			fixture := newStorageLaunchdFixture(t)
			ops := systemStorageServiceIO()
			fenced := window == "prepared" || window == "moving" || window == "parked"
			switch window {
			case "already admitted":
				oldLease, err := state.EnterStorageCommand()
				if err != nil {
					t.Fatal(err)
				}
				if rc := guidedDoctorStorageChange(fixture.manager, func([]int) error { return nil }, fixture.offline, func(string) bool { return true }); rc != 1 {
					t.Fatalf("migration returned %d with an already admitted writer", rc)
				}
				if _, parked, _, err := state.FindStorageServiceProfile(serveAgentLabel); err != nil || parked {
					t.Fatalf("a profile was parked before draining the old writer: %v", err)
				}
				if err := oldLease.Close(); err != nil {
					t.Fatal(err)
				}
			case "prepared":
				jobs, err := fixture.manager.Inspect()
				if err != nil {
					t.Fatal(err)
				}
				plan := &storageServicePlan{Phase: "pause", Jobs: jobs}
				save := func() error { return ops.save(plan) }
				for _, advance := range []func() error{
					save,
					func() error { return fixture.manager.Stop(jobs, save) },
					state.PrepareWorkflowServiceFence,
					func() error { return fixture.manager.Park(jobs, save) },
				} {
					if err := advance(); err != nil {
						t.Fatal(err)
					}
				}
			default:
				barrier := ops.barrier
				ops.barrier = func() error {
					if window == "parked" {
						return errors.New("simulated exit before returning profiles")
					}
					return barrier()
				}
				fixture.failBootstrap = window == "profile returned"
				checks := 0
				writers := func() error {
					checks++
					if window == "moving" && checks == 2 {
						return errors.New("simulated exit after publishing the layout plan")
					}
					return fixture.offline()
				}
				if rc := guidedDoctorStorageChangeWithIO(fixture.manager, func([]int) error { return nil }, writers, func(string) bool { return true }, ops); rc != 1 {
					t.Fatalf("expected prepared recovery, code %d", rc)
				}
			}
			if oldCLI != "" {
				fakeRestartedLaunchd(t, filepath.Join(t.TempDir(), "job-running"))
				output, err := exec.Command(oldCLI, "--lang=en", "tunnel", "--unservice").CombinedOutput()
				if fenced {
					if err == nil || !strings.Contains(string(output), "storage unavailable") {
						t.Fatalf("installed baseline falsely acknowledged remote off: %v %s", err, output)
					}
					if output, err := exec.Command(oldCLI, "--lang=en", "doctor", "--group-storage").CombinedOutput(); err == nil ||
						!strings.Contains(string(output), "unknown workflow layout journal") {
						t.Fatalf("installed baseline could bypass the service fence: %v %s", err, output)
					}
				} else if err != nil || !strings.Contains(string(output), "Remote access disabled") {
					t.Fatalf("installed baseline could not actually close access: %v %s", err, output)
				}
			} else if !fenced {
				// The baseline Off operation removes existing profiles.
				manager := userservice.Launchd{Home: state.Home()}
				for _, label := range []string{serveAgentLabel, selfTunnelAgentLabel} {
					if err := os.Remove(manager.Path(label)); err != nil {
						t.Fatal(err)
					}
				}
			}
			if fenced {
				if lease, err := state.EnterStorageCommand(); err == nil {
					lease.Close()
					t.Fatal("ordinary command admitted while a profile was parked")
				}
				if _, parked, _, err := state.FindStorageServiceProfile(serveAgentLabel); err != nil || !parked {
					t.Fatalf("parked profile changed during rejected off: %v", err)
				}
			}
			before := len(fixture.mutations)
			fixture.failBootstrap = false
			if rc := guidedDoctorStorageChange(fixture.manager, func([]int) error { return nil }, func() error { return nil }, func(string) bool { return true }); rc != 0 {
				t.Fatalf("recovery returned %d", rc)
			}
			if fenced {
				if !slices.Equal(fixture.mutations[before:], []string{"bootstrap:" + serveAgentLabel}) {
					t.Fatalf("expected one restore after a rejected off: %v", fixture.mutations)
				}
			} else if fixture.loaded || len(fixture.mutations) != before {
				t.Fatalf("real off was undone during recovery: loaded=%v actions=%v", fixture.loaded, fixture.mutations)
			}
		})
	}
}

func TestDoctorStorageCleanupRetryThroughCLI(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS Doctor entry")
	}
	fixture := newStorageLaunchdFixture(t)
	ops := systemStorageServiceIO()
	ops.sync = func(path string) error {
		if path == state.StorageServiceRecoveryDir() {
			return errors.New("injected sync failure after unlink")
		}
		return state.SyncPrivateDirectory(path)
	}
	if rc := guidedDoctorStorageChangeWithIO(fixture.manager, func([]int) error { return nil }, fixture.offline, func(string) bool { return true }, ops); rc != 1 {
		t.Fatalf("fault returned %d", rc)
	}
	t.Setenv("PATH", t.TempDir())
	output := runStorageDoctorPTY(t, "", func() {
		if rc := Run([]string{"--lang=en", "doctor", "--fix"}); rc != 0 {
			t.Fatalf("cleanup returned %d", rc)
		}
		if _, handled := doctorStorageEntry("doctor", []string{"--fix"}); handled {
			t.Fatal("completed cleanup intercepted the next normal Doctor repair")
		}
	})
	if !strings.Contains(output, "recovery cleanup finished") {
		t.Fatalf("cleanup result missing: %s", output)
	}
}

func TestDoctorStorageCustomCLIHelpPreservesIdentity(t *testing.T) {
	fixture := newStorageLaunchdFixture(t)
	fixture.loaded = false
	manager := userservice.Launchd{Home: state.Home()}
	binary := filepath.Join(state.Home(), "custom-bin", "gtmux")
	if err := manager.Write(manager.Path(serveAgentLabel), serveAgentLabel, []string{binary, "serve"}, filepath.Join(state.Dir(), "serve.log")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.manager.Inspect(); err == nil || !strings.Contains(err.Error(), "GTMUX_BIN_DIR") {
		t.Fatalf("custom install guidance missing: %v", err)
	}
	fixture.manager.Executable = binary
	if jobs, err := fixture.manager.Inspect(); err != nil || len(jobs) != 2 {
		t.Fatalf("registering CLI refused its own valid profile: %+v, %v", jobs, err)
	}
}
