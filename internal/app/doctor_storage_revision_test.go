package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
	"github.com/chenchaoyi/gtmux/internal/userservice"
)

func TestDoctorStorageRejectsMismatchedLaunchdProgram(t *testing.T) {
	for _, alternate := range []string{"installed program", "loaded program"} {
		t.Run(alternate, func(t *testing.T) {
			fixture := newStorageLaunchdFixture(t)
			if alternate == "installed program" {
				manager := userservice.Launchd{Home: state.Home()}
				if err := manager.Write(manager.Path(serveAgentLabel), serveAgentLabel,
					[]string{"/usr/bin/true", "serve"}, filepath.Join(state.Dir(), "serve.log")); err != nil {
					t.Fatal(err)
				}
			} else {
				fixture.foreignProgram = true
			}
			confirms := 0
			code := guidedDoctorStorageChange(fixture.manager, func([]int) error { return nil }, fixture.offline,
				func(string) bool { confirms++; return true })
			if code != 1 || confirms != 0 || len(fixture.mutations) != 0 || !fixture.loaded {
				t.Fatalf("foreign program admitted: rc=%d confirms=%d mutations=%v", code, confirms, fixture.mutations)
			}
		})
	}
}

func TestDoctorStorageReturnsSameProfileAndDefaults(t *testing.T) {
	fixture := newStorageLaunchdFixture(t)
	path, _, original, err := state.FindStorageServiceProfile(serveAgentLabel)
	if err != nil || path == "" {
		t.Fatal(path, err)
	}
	uid := strconv.Itoa(os.Getuid())
	// A healthy GUI job can have an explicit value only in the GUI domain.
	fixture.overrides["gui/"+uid+"/"+serveAgentLabel] = false
	if rc := guidedDoctorStorageChange(fixture.manager, func([]int) error { return nil }, fixture.offline, func(string) bool { return true }); rc != 0 {
		t.Fatalf("grouping returned %d", rc)
	}
	finalPath, parked, now, err := state.FindStorageServiceProfile(serveAgentLabel)
	jobs, inspectErr := fixture.manager.Inspect()
	if err != nil || path != finalPath || parked || now != original || inspectErr != nil ||
		len(jobs) != 2 || jobs[0].Override != [2]string{"enabled", "default"} ||
		!slices.Equal(fixture.mutations, []string{"bootout:" + serveAgentLabel, "bootstrap:" + serveAgentLabel}) {
		t.Fatalf("profile or overrides changed: %s %v %v, %+v %v, %v", finalPath, parked, err, jobs, inspectErr, fixture.mutations)
	}
}

func TestDoctorStoragePreservesChoiceDuringConfirmation(t *testing.T) {
	for _, change := range []string{"remote off", "new legacy login", "old menu process"} {
		t.Run(change, func(t *testing.T) {
			fixture := newStorageLaunchdFixture(t)
			if change == "old menu process" {
				bin := t.TempDir()
				t.Setenv("PATH", bin)
				// Verify the production process parser, including the legacy name.
				script := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '%d 73 /Applications/Gtmux.app/Contents/MacOS/gtmux-menubar'\n", os.Getuid())
				if err := os.WriteFile(filepath.Join(bin, "ps"), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
				if err := checkStorageProcesses([]int{73}); err == nil || !strings.Contains(err.Error(), "73") {
					t.Fatalf("legacy menu process was ignored: %v", err)
				}
				return
			}
			code := guidedDoctorStorageChange(fixture.manager, func([]int) error { return nil }, fixture.offline, func(string) bool {
				// The production remote config lock must be available while awaiting consent.
				unlock, err := lockRemoteServices()
				if err != nil {
					t.Fatal(err)
				}
				defer unlock()
				if change == "remote off" {
					uid := strconv.Itoa(os.Getuid())
					fixture.overrides["gui/"+uid+"/"+serveAgentLabel] = true
					fixture.loaded = false
				} else {
					fixture.legacyMenu = true
				}
				return true
			})
			if code != 1 || len(fixture.mutations) != 0 {
				t.Fatalf("settings changes admitted: rc=%d mutations=%v", code, fixture.mutations)
			}
			status, err := state.ReadStorageLayout()
			if err != nil || status.Workflow != "" || change == "remote off" && fixture.loaded {
				t.Fatalf("new choice overwritten: %+v, %v, loaded=%v", status, err, fixture.loaded)
			}
		})
	}
}

func TestDoctorStorageDurableBoundaries(t *testing.T) {
	for _, failure := range []string{"plan ancestor", "layout commit", "unlink", "cleanup sync"} {
		t.Run(failure, func(t *testing.T) {
			fixture := newStorageLaunchdFixture(t)
			ops := systemStorageServiceIO()
			inject := errors.New("injected durability failure")
			switch failure {
			case "plan ancestor":
				ops.sync = func(path string) error {
					if path == filepath.Dir(state.StorageServiceRecoveryDir()) {
						return inject
					}
					return state.SyncPrivateDirectory(path)
				}
			case "layout commit":
				ops.barrier = func() error { return inject }
			case "unlink":
				ops.remove = func(string) error { return inject }
			case "cleanup sync":
				ops.sync = func(path string) error {
					if path == state.StorageServiceRecoveryDir() {
						return inject
					}
					return state.SyncPrivateDirectory(path)
				}
			}
			processes := func([]int) error { return nil }
			confirm := func(string) bool { return true }
			if rc := guidedDoctorStorageChangeWithIO(fixture.manager, processes, fixture.offline, confirm, ops); rc != 1 {
				t.Fatalf("fault returned %d", rc)
			}
			plan, err := readStorageServicePlan()
			if err != nil {
				t.Fatal(err)
			}
			switch failure {
			case "plan ancestor":
				if len(fixture.mutations) != 0 || plan == nil || plan.Jobs[0].Stage != "" {
					t.Fatalf("paused before plan publication: %v %+v", fixture.mutations, plan)
				}
			case "layout commit":
				if fixture.loaded || plan == nil || !slices.Equal(fixture.mutations, []string{"bootout:" + serveAgentLabel}) {
					t.Fatalf("service restarted before layout barrier: %v %+v", fixture.mutations, plan)
				}
				if unlock, err := lockRemoteServices(); err == nil {
					unlock()
					t.Fatal("remote settings were writable while the original service plan was pending")
				}
			case "unlink":
				if plan == nil || plan.Phase != "verified" {
					t.Fatalf("verified cleanup witness missing: %+v", plan)
				}
			case "cleanup sync":
				if plan != nil {
					t.Fatalf("unlink before failed sync was not observable: %+v", plan)
				}
				if _, err := os.Stat(state.StorageServiceRecoveryDir()); err != nil {
					t.Fatalf("cleanup retry anchor missing: %v", err)
				}
			}
			if failure == "cleanup sync" {
				// This is the empty-anchor branch used by a rerun of Doctor.
				if err := syncStorageRecoveryAnchor(); err != nil {
					t.Fatal(err)
				}
			} else if rc := guidedDoctorStorageChange(fixture.manager, processes, fixture.offline, confirm); rc != 0 {
				t.Fatalf("rerun returned %d", rc)
			}
			if !slices.Equal(fixture.mutations, []string{"bootout:" + serveAgentLabel, "bootstrap:" + serveAgentLabel}) {
				t.Fatalf("retry replayed or skipped a service action: %v", fixture.mutations)
			}
			fixture.assertRestored()
		})
	}
}

func TestDoctorStorageRecoversPartialServicesAndBoundsKickstart(t *testing.T) {
	t.Run("dependent tunnel waits but disabled job can be restored", func(t *testing.T) {
		for _, directOn := range []bool{true, false} {
			t.Run(strconv.FormatBool(directOn), func(t *testing.T) {
				fixture := newStorageLaunchdFixture(t)
				if directOn {
					fixture.directLoaded = true
					uid := strconv.Itoa(os.Getuid())
					delete(fixture.overrides, "gui/"+uid+"/"+selfTunnelAgentLabel)
					delete(fixture.overrides, "user/"+uid+"/"+selfTunnelAgentLabel)
				}
				fixture.failBootstrap = true
				processes := func([]int) error { return nil }
				confirm := func(string) bool { return true }
				if rc := guidedDoctorStorageChange(fixture.manager, processes, fixture.offline, confirm); rc != 1 {
					t.Fatalf("bootstrap failure returned %d", rc)
				}
				_, parked, _, err := state.FindStorageServiceProfile(selfTunnelAgentLabel)
				if err != nil || parked != directOn || fixture.directLoaded {
					t.Fatalf("independent profile handling: parked=%v loaded=%v err=%v", parked, fixture.directLoaded, err)
				}
				fixture.failBootstrap = false
				if rc := guidedDoctorStorageChange(fixture.manager, processes, fixture.offline, confirm); rc != 0 ||
					!fixture.loaded || fixture.directLoaded != directOn {
					t.Fatalf("partial recovery incomplete: rc=%d loaded=%v direct=%v", rc, fixture.loaded, fixture.directLoaded)
				}
			})
		}
	})
	t.Run("loaded without a pid", func(t *testing.T) {
		fixture := newStorageLaunchdFixture(t)
		ops := systemStorageServiceIO()
		barrier := ops.barrier
		ops.barrier = func() error {
			fixture.idle = true
			return barrier()
		}
		processes := func([]int) error { return nil }
		confirm := func(string) bool { return true }
		if rc := guidedDoctorStorageChangeWithIO(fixture.manager, processes, fixture.offline, confirm, ops); rc != 1 {
			t.Fatalf("idle path returned %d", rc)
		}
		if rc := guidedDoctorStorageChange(fixture.manager, processes, fixture.offline, confirm); rc != 1 {
			t.Fatalf("idle retry returned %d", rc)
		}
		want := []string{"bootout:" + serveAgentLabel, "bootstrap:" + serveAgentLabel, "kickstart:" + serveAgentLabel}
		if !slices.Equal(fixture.mutations, want) {
			t.Fatalf("unexpected service retries %v, want %v", fixture.mutations, want)
		}
		fixture.idle = false
		if rc := guidedDoctorStorageChange(fixture.manager, processes, fixture.offline, confirm); rc != 0 {
			t.Fatalf("late natural start returned %d", rc)
		}
		fixture.assertRestored()
	})
}
