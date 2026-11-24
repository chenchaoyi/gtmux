package userservice

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestGSS005ManagerStorageReadiness(t *testing.T) {
	const label = "com.gtmux.serve"
	missing := exec.Command("sh", "-c", "exit 113").Run()
	launchdCases := []struct {
		name, output, want string
		err                error
	}{
		{"keepalive without pid", "state = waiting\npid = 0\n", "unload the launchd job", nil},
		{"absent", "Could not find service \"com.gtmux.serve\" in domain for uid: 501", "", missing},
		{"unverifiable", "operation not permitted: confidential args", "could not confirm", missing},
	}
	for _, tc := range launchdCases {
		t.Run(tc.name, func(t *testing.T) {
			err := stoppedLaunchdJob(func(name string, args ...string) ([]byte, error) {
				if name != "launchctl" || strings.Join(args, " ") != "print user/501/"+label {
					t.Fatalf("unexpected query: %s %v", name, args)
				}
				return []byte(tc.output), tc.err
			}, "user/501", label)
			wantStorageManagerResult(t, err, tc.want)
		})
	}
	systemdCases := []struct {
		name, output, want string
		err                error
	}{
		{"pending restart without pid", "LoadState=loaded\nActiveState=activating\nSubState=auto-restart\nUnitFileState=disabled\nJob=", "stop and disable", nil},
		{"enabled while inactive", "LoadState=loaded\nActiveState=inactive\nSubState=dead\nUnitFileState=enabled\nJob=", "stop and disable", nil},
		{"queued start while inactive", "LoadState=loaded\nActiveState=inactive\nSubState=dead\nUnitFileState=disabled\nJob=41", "stop and disable", nil},
		{"stopped and disabled", "LoadState=loaded\nActiveState=inactive\nSubState=dead\nUnitFileState=disabled\nJob=", "", nil},
		{"absent", "LoadState=not-found\nActiveState=inactive\nSubState=dead\nUnitFileState=\nJob=", "", nil},
		{"unreachable", "", "could not confirm", errors.New("failed to connect to bus")},
	}
	for _, tc := range systemdCases {
		t.Run(tc.name, func(t *testing.T) {
			err := stoppedSystemdUnit(func(name string, args ...string) ([]byte, error) {
				if name != "systemctl" || args[0] != "--user" || args[len(args)-1] != label+".service" {
					t.Fatalf("unexpected query: %s %v", name, args)
				}
				return []byte(tc.output), tc.err
			}, label+".service")
			wantStorageManagerResult(t, err, tc.want)
		})
	}
}

func wantStorageManagerResult(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("expected a stopped service: %v", err)
		}
		return
	}
	if err == nil || !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), "confidential") {
		t.Fatalf("expected %q without service arguments, got %v", want, err)
	}
}
