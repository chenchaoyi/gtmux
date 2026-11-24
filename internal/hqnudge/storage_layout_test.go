package hqnudge

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestGSS012StorageLayoutKeepsNativeWakeEvidence(t *testing.T) {
	intent, active, pin := fixtureNativePin(t)
	intent.Outcome = "uncertain"
	if err := writeNativeIntent(intent); err != nil {
		t.Fatal(err)
	}
	if err := settleNativeIntent(intent); err != nil {
		t.Fatal(err)
	}
	parked := claimBase(active) + nativeUncertainSuffix
	journal := nativeIntentPath(intent)
	known := make(map[string]os.FileInfo)
	data := make(map[string]string)
	for _, name := range []string{parked, pin, journal} {
		info, err := os.Lstat(name)
		if err != nil {
			t.Fatal(err)
		}
		bytes, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		data[name] = string(bytes)
		known[name] = info
	}
	if !os.SameFile(known[parked], known[pin]) {
		t.Fatal("fixture did not retain the exact parked inode at the pin")
	}
	if err := state.WritePrivateFile(state.StorePath("events.seq"), []byte("33")); err != nil {
		t.Fatal(err)
	}
	for _, grouped := range []bool{true, false} {
		if err := state.ChangeStorageLayout(grouped, func() error { return nil }); err != nil {
			t.Fatalf("layout change with parked wake evidence: %v", err)
		}
		if filepath.Dir(parked) != queueDir() || filepath.Dir(pin) != nativePinDir() || journal != nativeIntentPath(intent) {
			t.Fatal("wake readers changed paths")
		}
		for path, original := range known {
			check, err := os.Lstat(path)
			if err != nil || !os.SameFile(original, check) {
				t.Fatalf("migration changed %s: %v", filepath.Base(path), err)
			}
			if bytes, err := os.ReadFile(path); err != nil || string(bytes) != data[path] {
				t.Fatalf("migration changed wake bytes at %s: %v", filepath.Base(path), err)
			}
		}
		if _, err := os.Lstat(active); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("sending claim reappeared: %v", err)
		}
		if names := queuedNames(func(string) bool { return true }); len(names) != 0 {
			t.Fatal("migration made an unknown wake ready to send")
		}
		path := filepath.Join(state.Dir(), "events.seq")
		if grouped {
			path = filepath.Join(state.Dir(), "layout-v2", "events", "events.seq")
		}
		if data, err := os.ReadFile(path); err != nil || string(data) != "33" {
			t.Fatalf("ordinary movable store was not routed in the selected layout: %v", err)
		}
	}
}
