package userservice

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestGSS009InstalledLaunchAgentRequiresPersistentDisable(t *testing.T) {
	const label = "com.gtmux.serve"
	missing := exec.Command("sh", "-c", "exit 113").Run()
	cases := []struct {
		name, guiState, userState, want string
		installed, noGUI                bool
	}{
		{name: "no install, no GUI", noGUI: true},
		{name: "installed with missing GUI", installed: true, noGUI: true, want: "could not verify persistent"},
		{name: "installed and enabled", installed: true, guiState: "enabled", want: "persistently disable"},
		{name: "installed with no explicit override", installed: true, want: "persistently disable"},
		{name: "installed disabled in both domains", installed: true, guiState: "disabled", userState: "true"},
		{name: "installed enabled in user domain", installed: true, guiState: "disabled", userState: "enabled", want: "persistently disable"},
		{name: "conflicting overrides", installed: true, guiState: "enabled\n\"com.gtmux.serve\" => disabled", want: "persistently disable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			manager := Launchd{Home: t.TempDir()}
			if tc.installed {
				if err := os.MkdirAll(manager.Dir(), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(manager.Path(label), []byte("installed fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			err := stoppedLaunchdStorage(func(name string, args ...string) ([]byte, error) {
				if name != "launchctl" || len(args) != 2 {
					t.Fatalf("unexpected probe: %s %v", name, args)
				}
				gui := strings.HasPrefix(args[1], "gui/")
				if gui && tc.noGUI {
					return []byte("Could not find domain for user gui: 501"), missing
				}
				if args[0] == "print" {
					return []byte("Could not find service \"com.gtmux.serve\" in domain for uid: 501"), missing
				}
				if args[0] != "print-disabled" || !tc.installed {
					t.Fatalf("unexpected override query: %v", args)
				}
				state := tc.userState
				if gui {
					state = tc.guiState
				}
				return []byte("disabled services = {\n\"com.gtmux.serve\" => " + state + "\n}"), nil
			}, manager, "501", []string{label})
			wantStorageManagerResult(t, err, tc.want)
		})
	}
}

func TestGSS011SystemdRejectsAmbiguousQuiescence(t *testing.T) {
	const prefix = "LoadState=not-found\nActiveState=inactive\nSubState=dead\nJob=\n"
	for _, suffix := range []string{"", "garbled line", "UnitFileState=enabled\nUnitFileState="} {
		err := stoppedSystemdUnit(func(string, ...string) ([]byte, error) {
			return []byte(prefix + suffix), nil
		}, "com.gtmux.serve.service")
		wantStorageManagerResult(t, err, "could not confirm")
	}
}
