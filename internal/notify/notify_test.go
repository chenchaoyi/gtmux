package notify

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSendWritesQueueRequest verifies Send drops one complete, valid JSON
// request into the notify queue (the contract the menu-bar app drains) and
// leaves no temp file behind.
func TestSendWritesQueueRequest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	Send(Options{
		Kind:     "input",
		Title:    "Diting",
		Subtitle: "Claude Code",
		Message:  "Needs your input",
		Pane:     "%12",
		Session:  "Diting",
		IconPath: "/tmp/icon.png",
	})

	dir := filepath.Join(home, ".local", "share", "gtmux", "notify")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read notify dir: %v", err)
	}
	var jsons []string
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			t.Fatalf("leftover temp/hidden file: %s", e.Name())
		}
		if strings.HasSuffix(e.Name(), ".json") {
			jsons = append(jsons, e.Name())
		}
	}
	if len(jsons) != 1 {
		t.Fatalf("want 1 queued json, got %d (%v)", len(jsons), jsons)
	}

	data, err := os.ReadFile(filepath.Join(dir, jsons[0]))
	if err != nil {
		t.Fatal(err)
	}
	var got request
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("queued json is invalid: %v", err)
	}
	if got.Kind != "input" || got.Title != "Diting" || got.Pane != "%12" {
		t.Errorf("fields not preserved: %+v", got)
	}
	if got.Body != "Needs your input" || got.Icon != "/tmp/icon.png" {
		t.Errorf("body/icon not preserved: %+v", got)
	}
	if got.TS == 0 {
		t.Errorf("ts not stamped")
	}
}

// TestSanitizePaneID keeps the queue filename filesystem-safe.
func TestSanitizePaneID(t *testing.T) {
	if got := sanitize("%12"); got != "p12" {
		t.Errorf("sanitize(%%12) = %q, want p12", got)
	}
	if got := sanitize(""); got != "x" {
		t.Errorf("sanitize(empty) = %q, want x", got)
	}
}
