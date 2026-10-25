package state

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestMarkerWritersAreLocallyPrivate(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	for _, stale := range []bool{false, true} {
		paths := []string{ActivePath("%1"), WaitingPath("%1"), LastFinishedPath(), TabOrderPath()}
		if stale {
			for _, p := range paths {
				if err := os.Chmod(p, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}
		if err := Touch(paths[0]); err != nil {
			t.Fatal(err)
		}
		if err := WriteMarker(paths[1], "payload"); err != nil {
			t.Fatal(err)
		}
		if err := WriteLastFinished("%1"); err != nil {
			t.Fatal(err)
		}
		if err := SaveTabOrder([]string{"one", "two"}); err != nil {
			t.Fatal(err)
		}
		for _, p := range paths {
			if mode(t, p) != 0600 {
				t.Errorf("stale=%v %s: %o", stale, p, mode(t, p))
			}
		}
		if ReadMarker(paths[1]) != "payload" || ReadLastFinished() != "%1" {
			t.Fatal("payload changed")
		}
	}
}

func TestPrivateIOKeepsInodesFlagsAndForeignModes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := syscall.Umask(0)
	defer syscall.Umask(old)
	p := filepath.Join(Dir(), "locks", "counter")
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	f, err := OpenFile(p, os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if mode(t, p) != 0600 || mode(t, filepath.Dir(p)) != 0700 {
		t.Fatal("opened a wide inode")
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) {
		t.Fatal("replaced a lock inode")
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		t.Fatal(err)
	}
	defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	other, err := OpenFile(p, os.O_RDWR, 0600)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if err := syscall.Flock(int(other.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
		t.Fatal("lost shared lock inode")
	}
	if _, err := f.WriteString("new"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(p)
	if err != nil || string(data) != "oldnew" {
		t.Fatal("append changed", string(data), err)
	}
	if exclusive, err := OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600); err == nil {
		exclusive.Close()
		t.Fatal("lost O_EXCL")
	}
	foreign := filepath.Join(t.TempDir(), "external.conf")
	if err := WriteFile(foreign, []byte("first"), 0644); err != nil {
		t.Fatal(err)
	}
	if mode(t, foreign) != 0644 {
		t.Fatal("narrowed foreign creation")
	}
	if err := os.Chmod(foreign, 0640); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(foreign, []byte("next"), 0644); err != nil {
		t.Fatal(err)
	}
	if mode(t, foreign) != 0640 {
		t.Fatal("changed existing foreign mode")
	}
	link := filepath.Join(filepath.Dir(p), "link")
	if err := os.Symlink(foreign, link); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(link, []byte("do not truncate"), 0644); err == nil {
		t.Fatal("followed link")
	}
	data, err = os.ReadFile(foreign)
	if err != nil || string(data) != "next" {
		t.Fatal("truncated outside inode")
	}
}

func TestPrivateCreateFailureBoundaries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p := filepath.Join(Dir(), "missing", "leaf")
	if f, err := OpenFile(p, os.O_RDWR, 0600); !os.IsNotExist(err) {
		if f != nil {
			f.Close()
		}
		t.Fatalf("missing non-creation became success: %v", err)
	}
	parent, err := privateWriteParent(p)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(parent)
	if err := os.Remove(filepath.Dir(p)); err != nil {
		t.Fatal(err)
	}
	fd, err := openPrivateAt(parent, "leaf", os.O_WRONLY|os.O_CREATE, 0600)
	if err == nil {
		unix.Close(fd)
		t.Fatal("creation under a deleted directory succeeded")
	}
	if !os.IsNotExist(err) {
		t.Fatalf("lost ENOENT: %v", err)
	}
	if err := MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Dir(p), 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(p), 0700) })
	if f, err := OpenFile(p, os.O_CREATE|os.O_WRONLY, 0600); err == nil {
		f.Close()
		t.Fatal("swallowed a permission failure")
	}
}

func TestPrivateIOLeavesExternalAncestorsAndRootTargetsAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	ancestor := filepath.Dir(Dir())
	if err := os.MkdirAll(ancestor, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(ancestor, 0755); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(Dir(), []byte("not a file"), 0600); err == nil {
		t.Fatal("accepted managed root as a file")
	}
	if mode(t, ancestor) != 0755 {
		t.Fatal("narrowed an ancestor outside managed roots")
	}
	if err := WriteForeign(filepath.Join(ConfigDir(), "fixture"), []byte("private"), 0644); err != nil {
		t.Fatal(err)
	}
	if mode(t, filepath.Join(ConfigDir(), "fixture")) != 0600 {
		t.Fatal("foreign helper bypassed private policy")
	}
}

func TestPrivateIODefaultTemporaryDirectoryKeepsOSMeaning(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	if err := MkdirAll(Dir(), 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(Dir())
	if IsManagedPath("") {
		t.Fatal("empty temporary directory was interpreted as the managed cwd")
	}
	if err := MkdirAll("", 0700); !os.IsNotExist(err) {
		t.Fatalf("empty MkdirAll changed meaning: %v", err)
	}
	f, err := CreateTemp("", "private-policy-fixture-*")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())
	if filepath.Dir(f.Name()) != filepath.Clean(os.TempDir()) {
		t.Fatalf("changed default temp directory: %s", f.Name())
	}
}

func TestPolicyOpenDoesNotRepairReadOnlyInspection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	p := filepath.Join(Dir(), "metadata")
	if err := os.MkdirAll(Dir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(Dir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(p, 0644); err != nil {
		t.Fatal(err)
	}
	f, err := OpenFile(p, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if mode(t, p) != 0644 || mode(t, Dir()) != 0755 {
		t.Fatal("a read-only query repaired permissions")
	}
}
