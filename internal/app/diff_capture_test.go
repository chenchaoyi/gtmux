package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCommandPrefixChild(t *testing.T) {
	switch os.Getenv("GTMUX_TEST_PREFIX_CHILD") {
	case "large":
		for i := 0; i < 1024; i++ {
			fmt.Print(strings.Repeat("x", 4096))
		}
		os.Exit(0)
	case "wait":
		time.Sleep(time.Minute)
		os.Exit(0)
	}
}
func TestCommandPrefixBoundsAndStopsProducer(t *testing.T) {
	t.Setenv("GTMUX_TEST_PREFIX_CHILD", "large")
	out, cut, err := commandPrefix(3*time.Second, 4096, os.Args[0], "-test.run=^TestCommandPrefixChild$")
	if err != nil || !cut || len(out) != 4096 {
		t.Fatalf("prefix bytes=%d cut=%v err=%v", len(out), cut, err)
	}
	t.Setenv("GTMUX_TEST_PREFIX_CHILD", "wait")
	_, _, err = commandPrefix(50*time.Millisecond, 4096, os.Args[0], "-test.run=^TestCommandPrefixChild$")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout=%v", err)
	}
}
func TestDiffForCwdBoundsLargeDiffAndKeepsSmallOutput(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	dir := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		c := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false"}, args...)...)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git: %v %s", err, out)
		}
	}
	git("init")
	if err := os.WriteFile(filepath.Join(dir, "tracked"), []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	git("add", "tracked")
	git("commit", "-m", "base")
	if err := os.WriteFile(filepath.Join(dir, "tracked"), []byte("changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new-file"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	out, err := diffForCwd(dir)
	if err != nil || !strings.Contains(out, "+changed") || !strings.Contains(out, "+ new-file") || strings.Contains(out, "truncated") {
		t.Fatalf("small diff: %q %v", out, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tracked"), []byte(strings.Repeat("large text\n", 200000)), 0600); err != nil {
		t.Fatal(err)
	}
	out, err = diffForCwd(dir)
	if err != nil || !strings.HasSuffix(out, "\n… (diff truncated)\n") || len(out) > maxDiffBytes+len("\n… (diff truncated)\n") {
		t.Fatalf("large diff bytes=%d err=%v", len(out), err)
	}
}
