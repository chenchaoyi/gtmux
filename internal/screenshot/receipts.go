package screenshot

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/state"
)

type Receipt struct {
	Version    int    `json:"schema_version"`
	ID         string `json:"request_id"`
	PayloadSHA string `json:"payload_sha256"`
	TargetSHA  string `json:"target_sha256"`
	ImageSHA   string `json:"image_sha256"`
	State      string `json:"last_known_state"`
	Confirmed  bool   `json:"confirmed"`
	ObservedAt string `json:"observed_at"`
	Reason     string `json:"reason,omitempty"`
}

func Digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }

// Claim publishes uncertainty BEFORE an input mutation can happen. A repeated
// request returns its prior receipt and never implicitly resubmits it.
func Claim(id, target, image, note string) (Receipt, bool, error) {
	var empty Receipt
	if !safeID(id) {
		return empty, false, Fail("invalid-request-id", "Use a stable 16–80 character request ID")
	}
	dir, err := PrivateDir()
	if err != nil {
		return empty, false, err
	}
	payload, _ := json.Marshal([]string{target, image, note})
	sum := Digest(payload)
	path := filepath.Join(dir, id+".json")
	if _, err := os.Lstat(path); err == nil {
		r, e := ReadReceipt(id)
		if e != nil {
			return empty, false, e
		}
		if r.PayloadSHA != sum {
			return empty, false, Fail("request-conflict", "This request ID belongs to a different image, note or target")
		}
		return r, false, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return empty, false, err
	}
	if len(entries) >= 1000 {
		return empty, false, Fail("receipt-budget-full", "Screenshot receipt budget is full; preserve unresolved receipts before retiring old metadata")
	}
	r := Receipt{Version: Version, ID: id, PayloadSHA: sum, TargetSHA: Digest([]byte(target)), ImageSHA: image, State: "unknown", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Reason: "submission-not-yet-confirmed"}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if os.IsExist(err) {
		r, e := ReadReceipt(id)
		if e != nil {
			return empty, false, e
		}
		if r.PayloadSHA != sum {
			return empty, false, Fail("request-conflict", "This request ID belongs to another payload")
		}
		return r, false, nil
	}
	if err != nil {
		return empty, false, err
	}
	data, _ := json.Marshal(r)
	_, err = f.Write(append(data, '\n'))
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err == nil {
		err = syncDirectory(dir)
	}
	if err != nil {
		return empty, false, Fail("receipt-unconfirmed", "Could not confirm a durable submission claim; do not replay automatically")
	}
	return r, true, nil
}

func ReadReceipt(id string) (Receipt, error) {
	var r Receipt
	if !safeID(id) {
		return r, Fail("invalid-request-id", "Invalid screenshot request ID")
	}
	dir, err := privateDir(false)
	if err != nil {
		return r, Fail("receipt-unavailable", "Receipt is missing; this is not proof of non-delivery")
	}
	if err = ownedJSON(filepath.Join(dir, id+".json"), &r); err != nil {
		return r, Fail("receipt-unavailable", "Receipt is missing or unreadable; this is not proof of non-delivery")
	}
	if r.Version != Version || r.ID != id || len(r.PayloadSHA) != 64 {
		return r, Fail("receipt-invalid", "Screenshot receipt metadata is invalid")
	}
	if !validReceiptState(r.State, r.Confirmed) {
		return r, Fail("receipt-invalid", "Screenshot receipt state is inconsistent")
	}
	return r, nil
}

func validReceiptState(status string, confirmed bool) bool {
	switch status {
	case "landed", "active_turn_accepted":
		return confirmed
	case "queued", "unknown", "blocked", "failed":
		return !confirmed
	}
	return false
}

func Finish(r Receipt, status, reason string, confirmed bool) (Receipt, error) {
	previous, err := ReadReceipt(r.ID)
	if err != nil {
		return r, err
	}
	if previous.PayloadSHA != r.PayloadSHA || previous.TargetSHA != r.TargetSHA || previous.ImageSHA != r.ImageSHA || !validReceiptState(status, confirmed) {
		return r, Fail("request-conflict", "Receipt identity changed")
	}
	r.State, r.Reason, r.Confirmed, r.ObservedAt = status, reason, confirmed, time.Now().UTC().Format(time.RFC3339Nano)
	dir, err := privateDir(false)
	if err != nil {
		return r, err
	}
	data, err := json.Marshal(r)
	if err != nil {
		return r, err
	}
	if err = state.WriteFile(filepath.Join(dir, r.ID+".json"), append(data, '\n'), 0600); err != nil {
		return r, Fail("receipt-unconfirmed", "Submission may have happened; its result could not be saved")
	}
	if err = syncDirectory(dir); err != nil {
		return r, Fail("receipt-unconfirmed", "Submission may have happened; receipt durability could not be confirmed")
	}
	return r, nil
}

func syncDirectory(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
