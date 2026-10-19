package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/terminalkeys"
	"github.com/chenchaoyi/gtmux/internal/tmux"
)

// Records argv only: it never contacts the operator's tmux socket.
func TestTerminalKeyTransportValidation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	log := filepath.Join(dir, "calls")
	fake := filepath.Join(dir, "tmux")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + log + "'\n[ \"$1\" = -u ] && shift\nif [ \"$1\" = display-message ]; then printf '%s\\t%s\\t%s\\t%s\\t%s\\t%s\\n' '%91' '12345' '/dev/fixture' 'shell' '$99' 'keyboard-fixture:0.0'; fi\n"
	if err := os.WriteFile(fake, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	old := tmux.Bin
	tmux.Bin = fake
	t.Cleanup(func() { tmux.Bin = old })
	target, err := terminalInputTarget("%91")
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(log, nil, 0600)
	if sendTerminalKey("%91", target.Binding, terminalkeys.Event{Key: "NotAKey"}, nil) == nil {
		t.Fatal("invalid key accepted")
	}
	b, _ := os.ReadFile(log)
	if len(b) != 0 {
		t.Fatal("invalid event invoked tmux")
	}
	if sendTerminalKey("%91", "replaced", terminalkeys.Event{Key: "F2"}, nil) == nil {
		t.Fatal("stale route accepted")
	}
	b, _ = os.ReadFile(log)
	if strings.Contains(string(b), "send-keys") {
		t.Fatal("stale route exited copy mode or sent a key")
	}
	for _, key := range []string{";", "-"} {
		os.WriteFile(log, nil, 0600)
		if err := sendTerminalKey("%91", target.Binding, terminalkeys.Event{Key: key}, nil); err != nil {
			t.Fatal(err)
		}
		b, _ = os.ReadFile(log)
		want := "-H 3b"
		if key == "-" {
			want = "-H 2d"
		}
		if !strings.Contains(string(b), want) || strings.Contains(string(b), " Enter") {
			t.Fatalf("unsafe argv: %s", b)
		}
	}
}
