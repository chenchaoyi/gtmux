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

	"github.com/chenchaoyi/gtmux/internal/state"
)

const (
	// Legacy writers counted names without reserving the remaining claim
	// and receipt names. Retain their unresolved input while giving the
	// active queue room for new sends.
	inputLimit = 8192
	// Keep the scan bound independent of admission and identical to the DSH
	// receiver bound. Reserved completion slots count toward inputLimit.
	inputScanLimit = 8192
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
		return names, ErrInputFull
	}
	return names, nil
}

func admitInput(dir string) error {
	names, err := inputNames(dir)
	if err != nil {
		return err
	}
	if len(names)*inputReservation+inputReservation <= inputLimit {
		return nil
	}
	if inputUsedSlots(dir, names)+inputReservation > inputLimit {
		return ErrInputFull
	}
	return nil
}

// Each indexed operation can occupy three more names (claim, staged receipt
// and newly linked receipt). Count orphan and unrelated names too. Anchor
// reservations to the index even if an unlocked v1 plugin changes payload names
// during enumeration.
func inputUsedSlots(dir string, names []string) int {
	used := len(names)
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
	return used
}

// InputSpoolUsage is a read-only sample of the active queue. An over-limit
// scan reports a lower bound for Entries and leaves UsedSlots unmeasured.
type InputSpoolUsage struct {
	Entries, ScanLimit   int
	UsedSlots, SlotLimit int
	Reservation          int
}

// InspectInputSpool reports active names and settlement reservations using
// the same accounting as new input admission. It does not create or prune
// the queue. Intake and settlement can change usage after it is sampled.
func InspectInputSpool() (InputSpoolUsage, error) {
	result := InputSpoolUsage{
		ScanLimit: inputScanLimit, SlotLimit: inputLimit, Reservation: inputReservation,
	}
	store, err := resolveInputStore(state.Dir())
	if err != nil || store.directory == (inputIdentity{}) {
		return result, err
	}
	names, err := inputNames(store.path)
	result.Entries = len(names)
	if err != nil {
		return result, err
	}
	result.UsedSlots = inputUsedSlots(store.path, names)
	return result, nil
}

// Reconcile only an addressed v1 record. Never infer delivery from age or a
// claim. Retire a settlement only after its exact evidence is durable.
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
	retention := 24 * time.Hour
	if r.DurableRetry {
		retention = stableReceiptRetention
	}
	if (!retireSettled || r.DurableRetry) && now.Sub(time.UnixMilli(settledAt)) < retention {
		// A committed, matching receipt can replace a settled claim, including the legacy DSH marker.
		body, err := q.read(claim, 2<<20)
		if err != nil || !q.missing(pending) || len(body) != 0 && !matchesInputBody(body, r) || q.syncDocument(file) != nil {
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
	if err := q.retireInput(index, r, receipt); err != nil {
		if errors.Is(err, ErrInputFull) || errors.Is(err, ErrInputArchiveIntegrity) {
			return false, nil
		}
		return false, err
	}
	return true, nil
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
	if err := q.recoverRetirement(); err != nil {
		if errors.Is(err, ErrInputArchiveIntegrity) {
			// Keep the checkpoint for repair. Healthy IDs can use any
			// available slots without running a second retirement.
			return nil
		}
		return err
	}
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
