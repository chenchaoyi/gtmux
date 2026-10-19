package app

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/terminalkeys"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// A private socket and raw receiver verify transport bytes, not agent execution.
// No application cursor or extended-key mode is enabled in this baseline.
func TestTerminalKeyRawPTY(t *testing.T) {
	bin, err := exec.LookPath("tmux")
	if err != nil {
		t.Skip("tmux unavailable")
	}
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 unavailable")
	}
	dir, err := os.MkdirTemp("/tmp", "gtk-keys-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	dir, err = filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", dir)
	t.Setenv("TMUX", "")
	receiver := filepath.Join(dir, "receiver.py")
	output := filepath.Join(dir, "bytes")
	ready := filepath.Join(dir, "ready")
	script := "import os,tty\ntty.setraw(0)\nopen(" + fmt.Sprintf("%q", ready) + ",'w').close()\nf=open(" + fmt.Sprintf("%q", output) + ",'ab',buffering=0)\nwhile True:\n f.write(os.read(0,4096))\n"
	if err := os.WriteFile(receiver, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	wrapper := filepath.Join(dir, "tmux")
	if err := os.WriteFile(wrapper, []byte("#!/bin/sh\nexec "+shellQuote(bin)+" -S "+shellQuote(filepath.Join(dir, "s"))+" -f /dev/null \"$@\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	tmux.Bin = wrapper
	t.Cleanup(func() { _ = tmux.OK("kill-server"); tmux.Bin = old })
	if !tmux.OK("new-session", "-d", "-s", "keys", shellQuote(py)+" "+shellQuote(receiver)) {
		t.Fatal("private tmux start failed")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(ready); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("receiver not ready")
		}
		time.Sleep(10 * time.Millisecond)
	}
	id := tmux.Display("keys", "#{pane_id}")
	target, err := terminalInputTarget(id)
	if err != nil {
		t.Fatal(err)
	}
	var expected []byte
	n := 0
	for _, key := range terminalkeys.Catalog() {
		for _, mods := range key.ModifierSets {
			want, ok := baselineKeyBytes(key.Key, mods)
			if !ok {
				t.Fatalf("missing byte expectation for %s %v", key.Key, mods)
			}
			if err := sendTerminalKey(id, target.Binding, terminalkeys.Event{Key: key.Key, Modifiers: mods}, nil); err != nil {
				t.Fatalf("%s %v: %v", key.Key, mods, err)
			}
			expected = append(expected, want...)
			until := time.Now().Add(time.Second)
			for {
				got, _ := os.ReadFile(output)
				if len(got) >= len(expected) {
					if !bytes.Equal(got, expected) {
						t.Fatalf("%s %v: got suffix %x, want %x", key.Key, mods, got[len(expected)-len(want):], want)
					}
					break
				}
				if time.Now().After(until) {
					t.Fatalf("%s %v: bytes missing", key.Key, mods)
				}
				time.Sleep(time.Millisecond)
			}
			n++
		}
	}
	if n < 400 {
		t.Fatalf("transport matrix unexpectedly small: %d", n)
	}
	if sendTerminalKey(id, target.Binding, terminalkeys.Event{Key: "NotAKey"}, nil) == nil {
		t.Fatal("unknown key accepted")
	}
	got, _ := os.ReadFile(output)
	if !bytes.Equal(got, expected) {
		t.Fatal("unknown key wrote bytes")
	}
	version, _ := exec.Command(bin, "-V").Output()
	t.Logf("%s: %d advertised key combinations verified", strings.TrimSpace(string(version)), n)
}

func baselineKeyBytes(key string, mods []string) ([]byte, bool) {
	control, alt, shift := false, false, false
	for _, m := range mods {
		control = control || m == "control"
		alt = alt || m == "alt"
		shift = shift || m == "shift"
	}
	modifier := 1
	if shift {
		modifier++
	}
	if alt {
		modifier += 2
	}
	if control {
		modifier += 4
	}
	var f int
	if _, err := fmt.Sscanf(key, "F%d", &f); err == nil && f >= 1 && f <= 12 {
		if f <= 4 {
			if modifier == 1 {
				return []byte{27, 'O', byte('P' + f - 1)}, true
			}
			return []byte(fmt.Sprintf("\x1b[1;%d%c", modifier, 'P'+f-1)), true
		}
		code := []int{15, 17, 18, 19, 20, 21, 23, 24}[f-5]
		if modifier == 1 {
			return []byte(fmt.Sprintf("\x1b[%d~", code)), true
		}
		return []byte(fmt.Sprintf("\x1b[%d;%d~", code, modifier)), true
	}
	if end, ok := map[string]byte{"ArrowUp": 'A', "ArrowDown": 'B', "ArrowRight": 'C', "ArrowLeft": 'D'}[key]; ok {
		if modifier == 1 {
			return []byte{27, '[', end}, true
		}
		return []byte(fmt.Sprintf("\x1b[1;%d%c", modifier, end)), true
	}
	if code, ok := map[string]int{"Home": 1, "End": 4, "Insert": 2, "Delete": 3, "PageUp": 5, "PageDown": 6}[key]; ok {
		if modifier == 1 {
			return []byte(fmt.Sprintf("\x1b[%d~", code)), true
		}
		if key == "Home" {
			return []byte(fmt.Sprintf("\x1b[1;%dH", modifier)), true
		}
		if key == "End" {
			return []byte(fmt.Sprintf("\x1b[1;%dF", modifier)), true
		}
		return []byte(fmt.Sprintf("\x1b[%d;%d~", code, modifier)), true
	}
	var b []byte
	if len(key) == 1 {
		c := key[0]
		if control {
			if c >= 'a' && c <= 'z' {
				c -= 32
			}
			c &= 31
		} else if shift && c >= 'a' && c <= 'z' {
			c -= 32
		}
		b = []byte{c}
	} else {
		if key == "Tab" && shift {
			return []byte("\x1b[Z"), true
		}
		c, ok := map[string]byte{"Tab": 9, "Enter": 13, "Escape": 27, "Backspace": 127, "Space": 32}[key]
		if !ok {
			return nil, false
		}
		if control {
			c = 0
		}
		b = []byte{c}
	}
	if alt {
		b = append([]byte{27}, b...)
	}
	return b, true
}
