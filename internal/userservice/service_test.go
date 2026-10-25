package userservice

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestSystemdLifecycleUsesUserManagerAndChecksReadback(t *testing.T) {
	var calls [][]string
	fail := ""
	m := Systemd{Home: t.TempDir(), Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("manager call is not bounded")
		}
		calls = append(calls, append([]string{name}, args...))
		if strings.Join(args, " ") == fail {
			return nil, errors.New("secret output must not escape")
		}
		return nil, nil
	}}
	if err := m.Check(); err != nil {
		t.Fatal(err)
	}
	if err := m.Start(m.Path("com.gtmux.serve")); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(calls, [][]string{
		{"systemctl", "--user", "show-environment"},
		{"systemctl", "--user", "daemon-reload"},
		{"systemctl", "--user", "enable", "--now", "com.gtmux.serve.service"},
		{"systemctl", "--user", "is-active", "--quiet", "com.gtmux.serve.service"},
	}) {
		t.Fatalf("unexpected manager commands: %v", calls)
	}
	fail = "--user is-active --quiet com.gtmux.serve.service"
	if err := m.Start(m.Path("com.gtmux.serve")); err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("false success or leaked output: %v", err)
	}
	fail = "--user show-environment"
	if err := m.Check(); err == nil || !strings.Contains(err.Error(), "foreground") {
		t.Fatalf("missing recovery guidance: %v", err)
	}
}

func TestSystemdArgumentsAndRecoveryAreLiteralAndPrivate(t *testing.T) {
	home := t.TempDir()
	m := Systemd{Home: home, SearchPath: "/some tools:/usr/bin", Socket: "/tmp/a socket,123,9"}
	path := m.Path("com.gtmux.tunnel")
	args := []string{"/usr/bin/true", "--token", "a%h$HOME\"\\<&\nvalue"}
	if err := m.Write(path, "com.gtmux.tunnel", args, filepath.Join(home, "log file")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0600 {
		t.Fatalf("unit permissions = %o", fi.Mode().Perm())
	}
	if !strings.Contains(string(data), "StandardOutput=append:"+filepath.Join(home, "log file")+"\n") {
		t.Fatal("log path was quoted like an argv word")
	}
	if !strings.Contains(string(data), "ExecStart=:") || !strings.Contains(string(data), "%%h$HOME") || !strings.Contains(string(data), "Restart=on-failure") {
		t.Fatalf("unsafe unit: %s", data)
	}
	spec, err := readUnit(data)
	if err != nil || !reflect.DeepEqual(spec.Args, args) || spec.Socket != "/tmp/a socket,0,0" {
		t.Fatalf("literal arguments lost: %+v %v", spec, err)
	}
	token, err := m.Connector(data)
	if err != nil || token != args[2] {
		t.Fatalf("connector: %q %v", token, err)
	}
	changed, err := m.ReplaceConnector(data, token, "next%$value")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Connector(changed); got != "next%$value" {
		t.Fatal("recovery lost replacement")
	}
	if _, err := m.ReplaceConnector(append(data, []byte("# modified\n")...), token, "next"); err == nil {
		t.Fatal("hand-edited unit overwritten")
	}
	if _, err := m.ReplaceConnector(data, "wrong", "next"); err == nil {
		t.Fatal("replaced unknown connector")
	}
}

func TestLaunchdRetainsExistingPlistContract(t *testing.T) {
	m := Launchd{Home: t.TempDir()}
	path := m.Path("com.gtmux.tunnel")
	if err := m.Write(path, "com.gtmux.tunnel", []string{"/bin/true", "--token", "old<&"}, "/tmp/log"); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	next, err := m.ReplaceConnector(b, "old<&", "new<&")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := m.Connector(next); got != "new<&" {
		t.Fatalf("connector=%q", got)
	}
	if !strings.HasSuffix(path, "Library/LaunchAgents/com.gtmux.tunnel.plist") || !strings.Contains(string(b), "<key>KeepAlive</key><true/>") {
		t.Fatal("launchd contract changed")
	}
}

// This uses systemd's real parser on Linux CI; fixture-only rendering cannot
// establish whether quoting and log paths are accepted by the service manager.
func TestSystemdUnitPassesNativeVerifier(t *testing.T) {
	analyzer, err := exec.LookPath("systemd-analyze")
	if err != nil {
		t.Skip("systemd-analyze not installed on this host")
	}
	home := t.TempDir()
	m := Systemd{Home: home, SearchPath: "/usr/bin:/bin"}
	path := m.Path("com.gtmux.verify")
	if err := m.Write(path, "com.gtmux.verify", []string{"/usr/bin/true", "%h$HOME", "two words"}, filepath.Join(home, "log file")); err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command(analyzer, "verify", path).CombinedOutput()
	if err != nil {
		t.Fatalf("systemd rejected generated unit: %v\n%s", err, out)
	}
}

func TestServiceDescriptorsPrepareManagedCapturePrivately(t *testing.T) {
	for _, platform := range []string{"launchd", "systemd"} {
		t.Run(platform, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			previous := syscall.Umask(0)
			defer syscall.Umask(previous)
			var manager Manager = Launchd{Home: state.Home()}
			if platform == "systemd" {
				manager = Systemd{Home: state.Home()}
			}
			path := manager.Path("com.gtmux.fixture")
			log := state.CapturePath("serve")
			for _, stale := range []bool{false, true} {
				if stale {
					if err := os.Chmod(log, 0644); err != nil {
						t.Fatal(err)
					}
				}
				if err := manager.Write(path, "com.gtmux.fixture", []string{"/fixture/gtmux", "serve"}, log); err != nil {
					t.Fatal(err)
				}
				info, err := os.Stat(log)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatalf("capture stale=%v: %v %v", stale, info, err)
				}
				// Preparing a descriptor never starts/restarts a daemon, and existing output survives.
				if !stale {
					if err := os.WriteFile(log, []byte("previous output"), 0600); err != nil {
						t.Fatal(err)
					}
				} else {
					data, err := os.ReadFile(log)
					if err != nil || string(data) != "previous output" {
						t.Fatalf("truncated capture: %q %v", data, err)
					}
				}
			}

		})
	}
}
