package limits

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/chenchaoyi/gtmux/internal/dsh"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Wallet amounts are authoritative decimal strings, never a percentage or a price estimate.
type Wallet struct {
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
	Kind     string `json:"kind"` // recharge or bonus
}

type Balance struct {
	Agent      string   `json:"agent"`
	AgentName  string   `json:"agent_name"`
	AccountID  string   `json:"account_id"` // opaque digest; no contact or raw account ID
	Wallets    []Wallet `json:"wallets"`
	ObservedAt int64    `json:"observed_at"`
}

type accountObservation struct {
	SourceID   string   `json:"source_id"`
	Generation int64    `json:"generation"`
	Status     string   `json:"status"`
	AccountID  string   `json:"account_id,omitempty"`
	Wallets    []Wallet `json:"wallets,omitempty"`
	At         int64    `json:"at"`
}

type accountCache struct {
	Version int                  `json:"version"`
	Home    string               `json:"home"`
	Sources []accountObservation `json:"sources"`
}

var accountHex = regexp.MustCompile(`^[a-f0-9]{64}$`)
var sourceHex = regexp.MustCompile(`^[a-f0-9]{32}$`)
var moneyAmount = regexp.MustCompile(`^-?(?:0|[1-9][0-9]{0,19})(?:\.[0-9]{1,12})?$`)

func accountPath() string {
	return filepath.Join(state.Dir(), "account-usage", "dsh-"+dsh.HomeID()+".json")
}

func readAccountCache() accountCache {
	var c accountCache
	dir, err := os.Lstat(filepath.Dir(accountPath()))
	if err != nil || !dir.IsDir() || dir.Mode().Perm()&0077 != 0 {
		return c
	}
	leaf, err := os.Lstat(accountPath())
	if err != nil || !leaf.Mode().IsRegular() || leaf.Mode().Perm()&0077 != 0 {
		return c
	}
	f, err := os.Open(accountPath())
	if err != nil {
		return c
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || fi.Size() > 64<<10 {
		return c
	}
	if !os.SameFile(leaf, fi) || json.NewDecoder(f).Decode(&c) != nil || c.Version != 1 || c.Home != dsh.HomeID() || len(c.Sources) > 8 {
		return accountCache{}
	}
	for _, v := range c.Sources {
		if !validObservation(v) {
			return accountCache{}
		}
	}
	return c
}

func validObservation(next accountObservation) bool {
	if !sourceHex.MatchString(next.SourceID) || next.Generation < 1 {
		return false
	}
	switch next.Status {
	case "ready", "signed-out", "unavailable", "unsupported", "unverified-account":
	default:
		return false
	}
	if next.AccountID != "" && !accountHex.MatchString(next.AccountID) {
		return false
	}
	if next.Status != "ready" {
		return len(next.Wallets) == 0
	}
	if next.AccountID == "" || len(next.Wallets) == 0 || len(next.Wallets) > 8 {
		return false
	}
	seen := map[string]bool{}
	for _, w := range next.Wallets {
		key := w.Currency + ":" + w.Kind
		if seen[key] || (w.Currency != "CNY" && w.Currency != "USD") || (w.Kind != "recharge" && w.Kind != "bonus") || !moneyAmount.MatchString(w.Amount) {
			return false
		}
		seen[key] = true
	}
	return true
}

// ObserveDSHAccount accepts a bounded metadata-only hook. It has no lifecycle,
// credential, account-login, model, process-control or permission authority.
func ObserveDSHAccount(raw []byte, now time.Time) {
	if len(raw) > 64<<10 {
		return
	}
	var next accountObservation
	if json.Unmarshal(raw, &next) != nil || !validObservation(next) {
		return
	}

	lease, err := state.TryLease("dsh-account-usage")
	if err != nil {
		return
	}
	defer lease.Close()
	c := readAccountCache()
	c.Version, c.Home = 1, dsh.HomeID()
	values := make([]accountObservation, 0, 8)
	for _, old := range c.Sources {
		if old.SourceID == next.SourceID && old.Generation > next.Generation {
			return
		}
		if old.SourceID == next.SourceID && (old.AccountID == next.AccountID || next.AccountID == "") {
			continue
		}
		if now.UnixMilli()-old.At < 15*60*1000 && old.At <= now.UnixMilli() {
			values = append(values, old)
		}
	}
	next.At = now.UnixMilli()
	values = append(values, next)
	if len(values) > 8 {
		values = values[len(values)-8:]
	}
	c.Sources = values
	b, err := json.Marshal(c)
	if err != nil {
		return
	}
	dir := filepath.Dir(accountPath())
	if state.MkdirAll(dir, 0700) != nil {
		return
	}
	fi, err := os.Lstat(dir)
	if err != nil || !fi.IsDir() || fi.Mode().Perm()&0077 != 0 {
		return
	}
	f, err := state.CreateTemp(dir, ".dsh-usage-*")
	if err != nil {
		return
	}
	defer os.Remove(f.Name())
	if f.Chmod(0600) != nil {
		f.Close()
		return
	}
	_, err = f.Write(b)
	closeErr := f.Close()
	if err != nil || closeErr != nil {
		return
	}
	_ = os.Rename(f.Name(), accountPath())
}

// DSH balances are scoped to the current harness home and a verified native
// account digest. Identical accounts observed by Web and a TUI appear once.
func dshBalances(now time.Time) ([]Balance, string) {
	c := readAccountCache()
	by := map[string]accountObservation{}
	reason := "unsupported"
	for _, v := range c.Sources {
		if v.At > now.UnixMilli() || now.UnixMilli()-v.At > 15*60*1000 {
			continue
		}
		reason = v.Status
		if v.AccountID == "" {
			continue
		}
		if old, ok := by[v.AccountID]; !ok || v.At >= old.At {
			by[v.AccountID] = v
		}
	}
	var out []Balance
	for _, v := range by {
		if v.Status != "ready" {
			continue
		}
		out = append(out, Balance{Agent: "dsh", AccountID: v.AccountID, Wallets: v.Wallets, ObservedAt: v.At / 1000})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].AccountID < out[j].AccountID })
	return out, reason
}
