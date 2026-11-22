package hq

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/state"
)

func TestHND005SnapshotFailureDoesNotConsumeAlertOpportunity(t *testing.T) {
	sent := captureAttention(t)
	const now = 1300
	path := filepath.Join(state.NotifyDir(), "hq-relay.attention")
	if err := os.MkdirAll(path, 0700); err != nil {
		t.Fatal(err)
	}
	rows := []RelayAttention{{ID: "review", Since: 1000}}
	WatchRelayAttention(now, rows)
	if len(*sent) != 0 {
		t.Fatal("published a request without a verified attention snapshot")
	}
	cooldown := filepath.Join(state.Dir(), "hqwake", "notices-relay-continuation.json")
	if _, err := os.Stat(cooldown); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("state write failure consumed the alert opportunity", err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	WatchRelayAttention(now+1, rows)
	WatchRelayAttention(now+2, rows)
	if len(*sent) != 1 || (*sent)[0].AttentionID != "relay/review" {
		t.Fatalf("recovered store should present the ongoing report once: %#v", *sent)
	}
	requireAttentionSnapshot(t, "relay", `["relay/review"]`)
}
