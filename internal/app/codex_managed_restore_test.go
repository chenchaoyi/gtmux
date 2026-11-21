package app

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/resume"
)

func TestManagedRestorePreservesBridge(t *testing.T) {
	for _, scenario := range []string{"managed", "managed-symlink", "standalone", "wrapper", "external-server", "unknown-binary"} {
		t.Run(scenario, func(t *testing.T) {
			plan, pane, binary := cnrAppPlan(t)
			record := plan.Record
			supported := scenario == "managed" || scenario == "managed-symlink" || scenario == "standalone"
			oldInspect := managedRestoreInspectBinary
			t.Cleanup(func() { managedRestoreInspectBinary = oldInspect })
			managedRestoreInspectBinary = nativeRestoreInspectBinary
			if scenario != "standalone" {
				record.Frontend = strings.Repeat("a", 32)
			}
			if err := resume.Save(pane.Loc, record); err != nil {
				t.Fatal(err)
			}

			nativeRestoreBoot = resume.NativeRestoreBootID
			nativeRestoreReserve = resume.ReserveNativeRestoreAttempt
			nativeRestoreUpdate = resume.UpdateNativeRestoreAttempt
			sends := 0
			nativeRestoreSend = func(target, command string, enter bool) error {
				sends++
				if !supported {
					t.Fatal("unverified route wrote to the pane")
				}
				if target != pane.Pane || !enter {
					t.Fatalf("wrong pane or submission mode: %s, %v", target, enter)
				}
				occupied, err := resume.NativeRestoreOccupied(record)
				if err != nil || !occupied {
					t.Fatalf("used pane before durable reservation: occupied=%v err=%v", occupied, err)
				}
				commandRecord := record
				commandRecord.Cwd = plan.Identity.Cwd
				var want string
				if scenario != "standalone" {
					want = "cd -- '" + commandRecord.Cwd + "' && env CODEX_HOME='" + plan.Identity.Home + "' " +
						resume.CodexRestoreBinaryEnv + "='" + binary.Physical + "' '" + selfPath() +
						"' 'codex' '--no-alt-screen' 'resume' '" + record.SessionID + "'"
				} else {
					_, want, err = resume.NativeRestoreCommand(commandRecord, binary.Physical, plan.Identity.Home, true, "")
					if err != nil {
						t.Fatal(err)
					}
				}
				if command != want {
					t.Fatalf("got command %q, want %q", command, want)
				}
				return nil
			}
			if scenario != "standalone" {
				nativeRestoreProbe = func(context.Context, nativeRestoreBinary) nativeRestoreCapability {
					t.Fatal("managed route probed standalone mode")
					return nativeCapabilityUnknown
				}
			}
			switch scenario {
			case "managed-symlink":
				dir, err := filepath.EvalSymlinks(t.TempDir())
				if err != nil {
					t.Fatal(err)
				}
				binary.Lookup, binary.Physical = filepath.Join(dir, "codex"), filepath.Join(dir, "codex.js")
				if err := os.WriteFile(binary.Physical, []byte("#!/bin/sh\nexit 1\n"), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(binary.Physical, binary.Lookup); err != nil {
					t.Fatal(err)
				}
				managedRestoreInspectBinary = inspectManagedRestoreBinary
			case "wrapper":
				record.Launcher = "/custom/wrapper"
			case "external-server":
				t.Setenv("CODEX_EXEC_SERVER_URL", "https://example.invalid")
			case "unknown-binary":
				managedRestoreInspectBinary = func(string) (nativeRestoreBinary, bool, error) {
					return nativeRestoreBinary{}, false, nil
				}
			}

			dispatch := func() (resumeAttempt, bool) {
				return routeRestoreCommand(context.Background(), pane.Pane, pane.Loc, record,
					map[nativeRestoreBinary]nativeRestoreCapability{}, resumeAuto, map[string]bool{}, nil)
			}
			attempt, sent := dispatch()
			if !supported {
				if sent || sends != 0 || attempt.State != "managed-frontend-unavailable" {
					t.Fatalf("unverified route got %s, sent=%v, calls=%d", attempt.State, sent, sends)
				}
				return
			}
			if !sent || sends != 1 || attempt.State != "pending" || attempt.Native == nil {
				t.Fatalf("first dispatch got %s, sent=%v, calls=%d", attempt.State, sent, sends)
			}
			attempt, sent = dispatch()
			if sent || sends != 1 || attempt.State != "already-attempted" {
				t.Fatalf("same-boot dispatch got %s, sent=%v, calls=%d", attempt.State, sent, sends)
			}
		})
	}
}

func TestManagedRestoreNoStandaloneFallback(t *testing.T) {
	const child = "GTMUX_TEST_MANAGED_CODEX_CLI"
	if os.Getenv(child) == "1" {
		os.Exit(cmdCodex([]string{"--no-alt-screen", "resume", "fixture-thread"}))
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_DATA_HOME", filepath.Join(home, "data"))
	t.Setenv("CODEX_HOME", filepath.Join(home, "codex-home"))
	t.Setenv("PATH", home+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(child, "1")
	binary := filepath.Join(home, "codex")
	marker := filepath.Join(home, "standalone-executed")
	t.Setenv("GTMUX_TEST_CODEX_MARKER", marker)
	if err := os.WriteFile(binary, []byte("#!/bin/sh\n: > \"$GTMUX_TEST_CODEX_MARKER\"\nexit 91\n"), 0700); err != nil {
		t.Fatal(err)
	}

	launch := func(wantCode int) string {
		t.Helper()
		cmd := exec.Command(os.Args[0], "-test.run=^TestManagedRestoreNoStandaloneFallback$")
		output, err := cmd.CombinedOutput()
		var exit *exec.ExitError
		if !errors.As(err, &exit) || exit.ExitCode() != wantCode {
			t.Fatalf("child error %v, output %s, expected exit %d", err, output, wantCode)
		}
		return string(output)
	}
	t.Setenv(resume.CodexRestoreBinaryEnv, binary)
	if output := launch(1); !strings.Contains(output, "managed Codex restore could not start the terminal bridge") {
		t.Fatalf("managed failure missing: %s", output)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("managed launch reached the standalone executable: %v", err)
	}
	t.Setenv(resume.CodexRestoreBinaryEnv, "")
	if err := os.Unsetenv(resume.CodexRestoreBinaryEnv); err != nil {
		t.Fatal(err)
	}
	launch(91)
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("normal compatibility launch did not run: %v", err)
	}
}
