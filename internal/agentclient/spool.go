package agentclient

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	inputLimit = 4096
	// A previous writer may have admitted at the limit and still owe a
	// receipt per request. Bound enumeration, including that legacy overhang.
	inputScanLimit = 2 * inputLimit
	reclaimBatch   = 32
	// One index, plus claim and both names during exclusive receipt publication.
	inputReservation = 4
)

var (
	ErrInputFull      = errors.New("terminal input spool is full")
	ErrInputUncertain = errors.New("previous input has an uncertain outcome; do not retry automatically")
)

func inputNames(dir string) ([]string, error) {
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return inputDirectoryNames(f)
}

func inputDirectoryNames(f *os.File) ([]string, error) {
	names, err := f.Readdirnames(inputScanLimit + 1)
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(names) > inputScanLimit {
		return nil, ErrInputFull
	}
	return names, nil
}

// Each indexed operation can occupy three more names (claim, staged receipt
// and newly linked receipt). Count observed
// orphans and unrelated files too. Anchor reservations to the immutable index
// even if an unlocked v1 plugin changes payload names during enumeration.
func admitInput(dir string) error {
	names, err := inputNames(dir)
	if err != nil {
		return err
	}
	used := len(names)
	if used*inputReservation+inputReservation <= inputLimit {
		return nil
	}
	parts := make(map[string]int)
	receipts := make(map[string]Receipt)
	settled := make(map[string]bool)
	for _, name := range names {
		id, suffix, _ := strings.Cut(name, ".")
		if !instancePattern.MatchString(id) {
			continue
		}
		switch suffix {
		case "pending.json", "claimed.json":
			parts[id]++
		case "receipt.json":
			if receipt, ok := readInputReceipt(dir, id); ok {
				receipts[id] = receipt
			}
		}
	}
	for _, name := range names {
		key, ok := strings.CutSuffix(name, ".index.json")
		if !ok || !instancePattern.MatchString(key) {
			continue
		}
		var r Request
		if readInputRecord(filepath.Join(dir, name), &r, 8192, requestFields) != nil || r.Version != ProtocolVersion || !instancePattern.MatchString(r.ID) || r.CreatedAt <= 0 {
			used += inputReservation - 1
			continue
		}
		if receipt, ok := receipts[r.ID]; ok && receipt.matchesRequest(r) {
			settled[r.ID] = true
			continue
		}
		if _, observed := parts[r.ID]; !observed {
			used += inputReservation - 1
		}
	}
	for id, n := range parts {
		if !settled[id] {
			used += max(0, inputReservation-1-n)
		}
	}
	if used+inputReservation > inputLimit {
		return ErrInputFull
	}
	return nil
}

// Reconcile only an addressed v1 record. Never infer delivery from age or a
// claim. Remove the index last, and stop on a failed unlink.
func (q *inputQueue) reconcileInput(index string, now time.Time) (bool, error) {
	return q.reconcileInputAt(index, now, false)
}

func (q *inputQueue) reconcileInputAt(index string, now time.Time, retireSettled bool) (bool, error) {
	r, err := q.loadIndex(index)
	if err != nil {
		return false, nil
	}
	receipt, file, err := q.openReceipt(r)
	if err != nil {
		return false, nil
	}
	defer file.Close()
	if receipt.State == "uncertain" {
		return false, nil
	}
	pending, claim := r.ID+".pending.json", r.ID+".claimed.json"
	// A late receipt starts a fresh retention window. Older v1 fixtures may
	// omit its timestamp, in which case the request supplies the lower bound.
	settledAt := max(receipt.CreatedAt, r.CreatedAt)
	if r.DurableRetry || !retireSettled && now.Sub(time.UnixMilli(settledAt)) < 24*time.Hour {
		// Empty DSH claims remain replay markers through the retention window.
		body, err := q.read(claim, 2<<20)
		if err != nil || !q.missing(pending) || !matchesInputBody(body, r) || q.syncDocument(file) != nil {
			return false, nil
		}
		return true, q.remove(claim)
	}
	// An addressed receipt does not explain away a damaged or replaced body.
	if !q.reclaimableInputBody(pending, r, false) || !q.reclaimableInputBody(claim, r, true) || q.syncDocument(file) != nil {
		return false, nil
	}
	for _, name := range []string{pending, claim} {
		if err := q.remove(name); err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	// Persist removals of replayable payloads before retiring their receipt.
	if err := q.ops.syncDir(q.dir); err != nil {
		return false, err
	}
	if err := q.remove(index); err != nil {
		return false, err
	}
	if err := q.ops.syncDir(q.dir); err != nil {
		return false, err
	}
	if err := q.remove(file.Name()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, q.ops.syncDir(q.dir)
}

func (q *inputQueue) reclaimableInputBody(name string, r Request, allowLegacyClaim bool) bool {
	data, err := q.read(name, 2<<20)
	if err != nil {
		return errors.Is(err, os.ErrNotExist)
	}
	if len(data) == 0 {
		return allowLegacyClaim
	}
	return matchesInputBody(data, r)
}

func matchesInputBody(data []byte, r Request) bool {
	var body Request
	if decodeInputRecord(data, &body, requestFields) != nil || body.Text == "" {
		return false
	}
	sum := sha256.Sum256([]byte(body.Text))
	body.Text = ""
	return body == r && hex.EncodeToString(sum[:]) == r.TextHash
}

func (q *inputQueue) pruneInput(now time.Time) error {
	names, err := inputDirectoryNames(q.dir)
	if err != nil {
		return err
	}
	changed := 0
	for _, name := range names {
		key, ok := strings.CutSuffix(name, ".index.json")
		if !ok || !instancePattern.MatchString(key) {
			continue
		}
		done, err := q.reconcileInput(name, now)
		if err != nil {
			return err
		}
		if done {
			changed++
			if changed == reclaimBatch {
				break
			}
		}
	}
	return nil
}

type InputRefused struct{ Reason string }

func (e *InputRefused) Error() string { return e.Reason }

func missingInputFile(dir, id, suffix string) bool {
	_, err := os.Lstat(filepath.Join(dir, id+suffix))
	return os.IsNotExist(err)
}
