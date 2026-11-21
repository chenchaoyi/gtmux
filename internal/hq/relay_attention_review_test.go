package hq

import (
	"fmt"
	"strings"
	"testing"

	"github.com/chenchaoyi/gtmux/internal/notify"
)

func TestHRF001UnsurfacedRelayReferencesGetIndependentNotice(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	old := wakeDesktopNotice
	t.Cleanup(func() { wakeDesktopNotice = old })
	var delivered []string
	wakeDesktopNotice = func(options notify.Options) bool {
		delivered = append(delivered, options.Message)
		return true
	}
	var rows []RelayAttention
	for i := range 9 {
		rows = append(rows, RelayAttention{ID: fmt.Sprintf("report-%02d", i), Since: 1000})
	}
	WatchRelayAttention(1200, rows)
	if len(delivered) != 8 {
		t.Fatalf("first scan bypassed the notice budget: %d", len(delivered))
	}
	WatchRelayAttention(1230, rows)
	if len(delivered) != 9 || !strings.Contains(delivered[8], "gtmux relay show report-08") {
		t.Fatalf("unseen report was starved: %v", delivered)
	}
	WatchRelayAttention(1260, rows)
	if len(delivered) != 9 {
		t.Fatalf("fair scheduling discarded notice cooldown history: %v", delivered)
	}
}
