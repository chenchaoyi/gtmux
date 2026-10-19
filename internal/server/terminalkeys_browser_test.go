//go:build keyboard_browser_fixture

package server

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/chenchaoyi/gtmux/internal/terminalkeys"
)

// Run with -tags keyboard_browser_fixture -run TestTerminalKeysBrowserFixture -timeout 50m.
// Explicit opt-in browser fixture: real HTTP/auth/assets, synthetic pane and no tmux.
func TestTerminalKeysBrowserFixture(t *testing.T) {
	dir := os.Getenv("GTMUX_KEYBOARD_FIXTURE_DIR")
	if dir == "" {
		t.Fatal("GTMUX_KEYBOARD_FIXTURE_DIR is required")
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	screen := "Keyboard UI fixture (synthetic pane)\nWorking\n? 1 question\n⇧← to answer\n\n› Ask Codex to do anything\nCodex"
	enroll := NewEnrollManager(nil, nil)
	rows := []byte(`[{"agent":"Codex","agent_key":"codex","pane_id":"%1","session":"Keyboard UI fixture","window":"Terminal keys","status":"working","source":"tmux","task":"Synthetic input test","since":1}]`)
	s := New(Config{Token: "synthetic-fixture-owner"}, Deps{Enroll: enroll,
		AgentsJSON: func() ([]byte, error) { return rows, nil }, PanesJSON: func() ([]byte, error) { return rows, nil },
		PaneText:   func(string) (string, bool) { mu.Lock(); defer mu.Unlock(); return screen, true },
		Transcript: func(string, int) ([]byte, TranscriptMeta, error) { return []byte(`[]`), TranscriptMeta{}, nil },
		UsageJSON:  func() ([]byte, error) { return []byte(`{"windows":[]}`), nil }, DigestJSON: func() ([]byte, error) { return []byte(`{"rows":[]}`), nil },
		Host: func() any {
			return map[string]any{"name": "Keyboard fixture", "os": "darwin", "capabilities": map[string]bool{"desktop_focus": false}}
		},
		InputTarget: func(string) (terminalkeys.Target, error) {
			mu.Lock()
			defer mu.Unlock()
			return terminalkeys.Target{Binding: "synthetic-pane", Actions: terminalkeys.CodexQuestions("codex", screen, "synthetic-pane")}, nil
		},
		InputKey: func(id, binding string, e terminalkeys.Event, a *terminalkeys.ActionRequest) error {
			mu.Lock()
			defer mu.Unlock()
			line, _ := json.Marshal(map[string]any{"pane": id, "event": e, "action": a})
			f, err := os.OpenFile(filepath.Join(dir, "events.jsonl"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			defer f.Close()
			f.Write(append(line, '\n'))
			if a != nil {
				screen = "Keyboard UI fixture (synthetic question)\nChoose a test answer\n› 1. Continue\n  2. Stop\nEnter to select"
			} else {
				screen = fmt.Sprintf("Keyboard UI fixture\nReceived %s %v\n› Saved remote draft", e.Key, e.Modifiers)
			}
			return nil
		},
		Send: func(id, text, key string, enter bool, sendID string) error {
			return fmt.Errorf("fixture does not accept legacy or text input")
		},
	})
	web := httptest.NewUnstartedServer(s.Handler())
	if addr := os.Getenv("GTMUX_KEYBOARD_FIXTURE_ADDR"); addr != "" {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		web.Listener = listener
	}
	web.Start()
	defer web.Close()
	defer web.CloseClientConnections()
	info, _ := json.Marshal(map[string]string{"url": web.URL, "code": enroll.Mint()})
	if err := os.WriteFile(filepath.Join(dir, "fixture.json"), info, 0o600); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(45 * time.Minute)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(dir, "stop")); err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("browser fixture reached its 45 minute limit")
}
