package state

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"testing"
	"time"
)

func TestLeaseChild(t *testing.T) {
	if os.Getenv("GTMUX_TEST_LEASE_CHILD") != "1" {
		return
	}
	_, err := TryLease("test")
	if errors.Is(err, ErrLeaseHeld) {
		os.Exit(73)
	}
	if err != nil {
		os.Exit(74)
	}
	// Intentionally skip Close: kernel ownership must end with the process.
	os.Exit(0)
}

func TestLeaseExcludesProcessesAndRecoversAfterExit(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	lease, err := TryLease("test")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	child := func(want int) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestLeaseChild$")
		cmd.Env = append(os.Environ(), "GTMUX_TEST_LEASE_CHILD=1")
		out, err := cmd.CombinedOutput()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("lease child: %v %s", err, out)
			}
			code = exit.ExitCode()
		}
		if code != want {
			t.Fatalf("child exit=%d want=%d output=%s", code, want, out)
		}
	}
	child(73)
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	child(0)
	lease, err = TryLease("test")
	if err != nil {
		t.Fatalf("process exit left a stale lock: %v", err)
	}
	defer lease.Close()
}
