package app

import (
	"github.com/chenchaoyi/gtmux/internal/dispatch"
	"github.com/chenchaoyi/gtmux/internal/server"
	"testing"
	"time"
)

func TestMSD002PhoneInputOptsIntoDirectUserDeliveryAndKeepsDraftProtection(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	for _, id := range []string{"", "stable-phone-message"} {
		o := phoneDeliverOpts("%8", "codex", id, dispatch.Tuning{})
		if !o.DirectUserInput || o.ClobberDraft || o.DeliveryID != id {
			t.Fatal(o)
		}
	}
}

func TestMSD002PhoneReceiptCacheKeepsQueueMeaningAndExpiry(t *testing.T) {
	sendCacheMu.Lock()
	old := phoneReceipts
	phoneReceipts = map[string]phoneReceipt{}
	sendCacheMu.Unlock()
	t.Cleanup(func() { sendCacheMu.Lock(); phoneReceipts = old; sendCacheMu.Unlock() })
	key := "%1\x00same-id"
	rememberPhoneReceipt(key, server.InputReceipt{State: "queued", SendID: "same-id"})
	got, ok := readPhoneReceipt(key)
	if !ok || got.State != "queued" || got.SendID != "same-id" {
		t.Fatal(got, ok)
	}
	if _, ok := readPhoneReceipt("%2\x00same-id"); ok {
		t.Fatal("receipt crossed panes")
	}
	sendCacheMu.Lock()
	phoneReceipts[key] = phoneReceipt{receipt: got, at: time.Now().Unix() - sendCacheTTLSec - 1}
	sendCacheMu.Unlock()
	if _, ok := readPhoneReceipt(key); ok {
		t.Fatal("expired receipt revived")
	}
	rememberPhoneReceipt("fresh", server.InputReceipt{State: "accepted"})
	sendCacheMu.Lock()
	_, exists := phoneReceipts[key]
	sendCacheMu.Unlock()
	if exists {
		t.Fatal("expired receipt retained")
	}
}
