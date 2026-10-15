//go:build linux

package app

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/userservice"
)

func TestLinuxDoctorOnlyChecksInstalledUserServices(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GTMUX_LANG", "en")
	t.Setenv("XDG_CONFIG_HOME", "")
	old := serviceManager
	defer func() { serviceManager = old }()
	var calls []string
	manager := userservice.Systemd{Home: home, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, name+" "+strings.Join(args, " "))
		if args[len(args)-1] == serveAgentLabel+".service" {
			return []byte("active\n"), nil
		}
		return nil, errors.New("inactive or unavailable user manager")
	}}
	serviceManager = func() userservice.Manager { return manager }
	if rows := platformServiceChecks(); len(rows) != 0 || len(calls) != 0 {
		t.Fatalf("foreground-only host requires a user manager: rows=%+v calls=%v", rows, calls)
	}
	if err := os.MkdirAll(manager.Dir(), 0700); err != nil {
		t.Fatal(err)
	}
	for _, label := range []string{serveAgentLabel, selfTunnelAgentLabel} {
		if err := os.WriteFile(manager.Path(label), []byte("fixture"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	rows := platformServiceChecks()
	if len(rows) != 2 || rows[0].status != stOK || rows[1].status != stRec ||
		!strings.Contains(rows[1].note, "systemctl --user status "+selfTunnelAgentLabel+".service") {
		t.Fatalf("service failure was hidden or wrong remedy offered: %+v", rows)
	}
	for _, call := range calls {
		if !strings.HasPrefix(call, "systemctl --user is-active ") {
			t.Fatalf("read-only doctor mutated user services: %s", call)
		}
	}
}

func TestLinuxDoctorDesktopRepairsAreNotApplicable(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, row := range append(appChecks(), terminalChecks()...) {
		if row.status != stInfo {
			t.Fatalf("unsupported desktop feature offered as repair: %+v", row)
		}
	}
	if len(platformSleepChecks()) != 0 {
		t.Fatal("Linux host was checked for macOS lid settings")
	}
	if rc := cmdDoctor([]string{"--terminal", "--fix"}); rc != 2 {
		t.Fatalf("headless desktop repair exit=%d, want unsupported", rc)
	}
}
