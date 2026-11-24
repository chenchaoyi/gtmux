//go:build darwin || linux

package codexbridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/state"
)

func shortBridgeTestDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("/tmp", "gbr")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return dir
}

func TestRunKeepsStoreFailureOffTerminal(t *testing.T) {
	const fixtureFlag = "GTMUX_BRIDGE_TEST_RUN"
	if os.Getenv(fixtureFlag) == "1" {
		home := state.Home()
		frontendSocketDirectory = func() (string, error) { return filepath.Join(home, "front"), nil }
		frontendSyslogSockets = []string{filepath.Join(home, "syslog")}
		if err := os.MkdirAll(diag.DayFile(state.LogsDir(), time.Now().Format("2006-01-02")), 0700); err != nil {
			t.Fatal(err)
		}
		os.Exit(Run(filepath.Join(home, "frontend"), nil, "%999"))
	}
	for _, fixture := range []struct {
		name string
		exit int
	}{
		{"socket-ready", 37},
		{"socket-missing", 38},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			home := shortBridgeTestDir(t)
			if err := os.Mkdir(filepath.Join(home, "front"), 0700); err != nil {
				t.Fatal(err)
			}
			frontend := `#!/bin/sh
test "$1" = --remote && test -S "${2#unix://}" || exit 99
printf 'launch\n' >> "$HOME/launches"
printf 'frontend stdout\n'
printf 'frontend stderr\n' >&2
exit "$GTMUX_BRIDGE_TEST_EXIT"
`
			if err := os.WriteFile(filepath.Join(home, "frontend"), []byte(frontend), 0700); err != nil {
				t.Fatal(err)
			}

			var receiver *net.UnixConn
			if fixture.name == "socket-ready" {
				path := filepath.Join(home, "syslog")
				var err error
				receiver, err = net.ListenUnixgram("unixgram", &net.UnixAddr{Name: path, Net: "unixgram"})
				if err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() { _ = receiver.Close() })
			}

			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRunKeepsStoreFailureOffTerminal$")
			cmd.Env = append(os.Environ(), "HOME="+home, fixtureFlag+"=1",
				"GTMUX_BRIDGE_TEST_EXIT="+strconv.Itoa(fixture.exit))
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			stdout, err := cmd.Output()
			var exited *exec.ExitError
			if !errors.As(err, &exited) || exited.ExitCode() != fixture.exit {
				t.Fatalf("bridge exit = %v, want %d; stderr %q", err, fixture.exit, stderr.String())
			}
			if string(stdout) != "frontend stdout\n" || stderr.String() != "frontend stderr\n" {
				t.Fatalf("shared terminal stdout=%q stderr=%q, want only frontend output", stdout, stderr.String())
			}
			if launches, err := os.ReadFile(filepath.Join(home, "launches")); err != nil || string(launches) != "launch\n" {
				t.Fatalf("frontend launches = %q, %v; want one", launches, err)
			}
			if receiver != nil {
				if err := receiver.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				var packet [512]byte
				n, err := receiver.Read(packet[:])
				if err != nil || !strings.Contains(string(packet[:n]), "diagnostics store write failed") {
					t.Fatalf("private syslog warning = %q, %v", packet[:n], err)
				}
			}
		})
	}
}

func TestProtectedSendKeepsManuallyTypedComposerBytes(t *testing.T) {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux is unavailable")
	}
	for _, fixture := range []struct {
		name, draft string
	}{
		{"empty", ""},
		{"draft", "manual  草稿 (保留)"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			home := shortBridgeTestDir(t)
			t.Setenv("HOME", home)
			t.Setenv("SHELL", "/bin/sh")
			socket := filepath.Join(home, "tmux")
			receipt := filepath.Join(home, "submitted")
			script := filepath.Join(home, "composer")
			code := `printf '› '
IFS= read -r line || exit 1
printf '%s' "$line" > "$1"
exec sleep 10
`
			if err := os.WriteFile(script, []byte(code), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
			defer cancel()
			tmux := func(args ...string) string {
				t.Helper()
				args = append([]string{"-u", "-S", socket, "-f", "/dev/null"}, args...)
				b, err := exec.CommandContext(ctx, bin, args...).CombinedOutput()
				if err != nil {
					t.Fatalf("private tmux %q: %v (%s)", args, err, b)
				}
				return string(b)
			}
			t.Cleanup(func() {
				cleanup, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = exec.CommandContext(cleanup, bin, "-S", socket, "kill-server").Run()
			})
			tmux("new-session", "-d", "-s", "compose", "-x", "160", "-y", "16",
				fmt.Sprintf("/bin/sh %s %s", script, receipt))
			capture := func() string { return tmux("capture-pane", "-pe", "-t", "compose") }
			waitForBridgeFixture(t, ctx, func() bool {
				draft, structured := dispatch.DraftOfColored(capture())
				return structured && draft == ""
			})
			if fixture.draft != "" {
				tmux("send-keys", "-l", "-t", "compose", "--", fixture.draft)
				waitForBridgeFixture(t, ctx, func() bool {
					draft, structured := dispatch.DraftOfColored(capture())
					return structured && draft == fixture.draft
				})
			}
			var pastes, enters, clears int
			io := dispatch.IO{
				Capture: capture, CaptureColor: capture,
				Now:   func() int64 { return time.Now().Unix() },
				Sleep: func() { time.Sleep(15 * time.Millisecond) },
				Paste: func(text string) error {
					pastes++
					tmux("set-buffer", "-b", "fixture", "--", text)
					tmux("paste-buffer", "-p", "-d", "-b", "fixture", "-t", "compose")
					return nil
				},
				Enter: func() error {
					enters++
					tmux("send-keys", "-t", "compose", "Enter")
					return nil
				},
				ClearDraft: func() error {
					clears++
					tmux("send-keys", "-t", "compose", "C-u")
					return nil
				},
			}

			const payload = "protected send"
			result := dispatch.PasteAndSubmitResult(io, dispatch.Opts{Pane: "compose", HasComposer: true}, payload)
			want := payload
			if fixture.draft == "" {
				if !result.Submitted() || pastes != 1 || enters != 1 || clears != 0 {
					t.Fatalf("empty pane result=%+v paste=%d enter=%d clear=%d", result, pastes, enters, clears)
				}
			} else {
				if result.State != dispatch.StateRefusedDraft || pastes != 0 || enters != 0 || clears != 0 {
					t.Fatalf("draft result=%+v paste=%d enter=%d clear=%d", result, pastes, enters, clears)
				}
				if got, ok := dispatch.DraftOfColored(capture()); !ok || got != fixture.draft {
					t.Fatalf("draft changed to %q", got)
				}
				if _, err := os.Stat(receipt); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("refused draft already submitted: %v", err)
				}
				// A manual Enter after the refusal reads the actual PTY input bytes.
				tmux("send-keys", "-t", "compose", "Enter")
				want = fixture.draft
			}
			waitForBridgeFixture(t, ctx, func() bool {
				b, err := os.ReadFile(receipt)
				return err == nil && string(b) == want
			})
		})
	}
}

func waitForBridgeFixture(t *testing.T, ctx context.Context, ready func() bool) {
	t.Helper()
	for !ready() {
		select {
		case <-ctx.Done():
			t.Fatal("private terminal did not reach the expected state")
		case <-time.After(15 * time.Millisecond):
		}
	}
}
