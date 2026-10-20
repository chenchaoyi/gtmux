package limits

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDSHBalancesArePrivateScopedFreshAndInvalidateOnAccountSwitch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	now := time.Unix(2000000, 0)
	account := strings.Repeat("a", 64)
	source := strings.Repeat("b", 32)
	put := func(o accountObservation, at time.Time) {
		t.Helper()
		raw, _ := json.Marshal(o)
		ObserveDSHAccount(raw, at)
	}
	ready := accountObservation{SourceID: source, Generation: 1, Status: "ready", AccountID: account, Wallets: []Wallet{{Currency: "CNY", Amount: "12.345678", Kind: "recharge"}, {Currency: "CNY", Amount: "0", Kind: "bonus"}}}
	put(ready, now)
	b, _ := dshBalances(now)
	if len(b) != 1 || b[0].Wallets[0].Amount != "12.345678" {
		t.Fatal(b)
	}
	r, ok := Get(Config{}, false, now)
	if !ok || len(r.Balances) != 1 || r.Balances[0].AgentName != "DeepSeek Harness" {
		t.Fatal("balance-only report lost registry identity", r)
	}
	fi, _ := os.Stat(accountPath())
	if fi.Mode().Perm() != 0600 {
		t.Fatal(fi.Mode())
	}
	if b, _ := dshBalances(now.Add(16 * time.Minute)); len(b) != 0 {
		t.Fatal("stale balance shown")
	}
	other := ready
	other.SourceID = strings.Repeat("c", 32)
	put(other, now)
	if b, _ := dshBalances(now); len(b) != 1 {
		t.Fatal("duplicate account", b)
	}
	put(accountObservation{SourceID: source, Generation: 2, Status: "signed-out", AccountID: account}, now)
	if b, _ := dshBalances(now); len(b) != 0 {
		t.Fatal("logout retained another source's old balance", b)
	}
	put(ready, now.Add(time.Second))
	if b, _ := dshBalances(now.Add(time.Second)); len(b) != 0 {
		t.Fatal("late old generation restored account")
	}
	next := ready
	next.Generation = 3
	next.AccountID = strings.Repeat("d", 64)
	put(next, now.Add(2*time.Second))
	b, _ = dshBalances(now.Add(2 * time.Second))
	if len(b) != 1 || b[0].AccountID != next.AccountID {
		t.Fatal(b)
	}
	t.Setenv("DSH_HOME", filepath.Join(t.TempDir(), "other"))
	if b, _ := dshBalances(now.Add(2 * time.Second)); len(b) != 0 {
		t.Fatal("home scope crossed")
	}
}
func TestDSHAccountHookRejectsSecretsMalformedAndUnsafeFiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("DSH_HOME", t.TempDir())
	now := time.Now()
	for _, raw := range []string{`{"status":"ready","account_id":"raw-user","wallets":[]}`, strings.Repeat("x", 65537)} {
		ObserveDSHAccount([]byte(raw), now)
	}
	if _, err := os.Stat(accountPath()); !os.IsNotExist(err) {
		t.Fatal("invalid observation persisted")
	}
	o := accountObservation{SourceID: strings.Repeat("b", 32), Generation: 1, Status: "ready", AccountID: strings.Repeat("a", 64), Wallets: []Wallet{{Currency: "USD", Amount: "-0.01", Kind: "recharge"}}}
	raw, _ := json.Marshal(o)
	ObserveDSHAccount(raw, now)
	if b, _ := dshBalances(now); len(b) != 1 {
		t.Fatal("authoritative negative money rejected")
	}
	if err := os.Chmod(accountPath(), 0644); err != nil {
		t.Fatal(err)
	}
	if b, _ := dshBalances(now); len(b) != 0 {
		t.Fatal("public cache trusted")
	}
}
func TestStaleWindowsDoNotDriveAlertsOrSummaries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Unix(2000000, 0)
	save(Report{At: now.Add(-time.Hour).Unix(), Windows: []Window{{Agent: "claude", Label: "claude week", PctUsed: 100}}})
	cfg := DefaultConfig
	cfg.Command = ""
	r, _ := Get(cfg, false, now)
	if !r.Windows[0].Stale || r.Windows[0].Tier != "" || r.Warn != "" || len(Summary(r.Windows)) != 0 {
		t.Fatal(r)
	}
}
