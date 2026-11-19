package agentclient

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

const (
	stableReceiptRetention = 72 * time.Hour
	retiredInputDir        = ".retired"
	retirementTail         = "tail.json"
	retirementTailLimit    = 20 << 10
	retiredShardLimit      = 256 << 10 // 256 shards, at most 64 MiB of exact IDs.
)

type inputRetirement struct {
	Index   string  `json:"index"`
	Request Request `json:"request"`
	Receipt Receipt `json:"receipt"`
}

func (q *inputQueue) openRetired(create bool) (*inputQueue, error) {
	if create {
		if err := unix.Mkdirat(q.fd(), retiredInputDir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return nil, err
		}
	}
	f, err := q.open(retiredInputDir, unix.O_RDONLY|unix.O_DIRECTORY)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err == nil {
		_, _, err = privateInputIdentity(info)
	}
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &inputQueue{dir: f, ops: q.ops}, nil
}

func retiredKey(index string) (string, [16]byte, bool) {
	var id [16]byte
	key, ok := strings.CutSuffix(index, ".index.json")
	if !ok || !instancePattern.MatchString(key) {
		return "", id, false
	}
	if _, err := hex.Decode(id[:], []byte(key)); err != nil {
		return "", id, false
	}
	return key[:2] + ".ids", id, true
}

func containsRetiredID(shard []byte, id [16]byte) (bool, error) {
	if len(shard)%len(id) != 0 {
		return false, ErrInputUncertain
	}
	for len(shard) != 0 {
		if [16]byte(shard[:16]) == id {
			return true, nil
		}
		shard = shard[16:]
	}
	return false, nil
}

func (q *inputQueue) retiredAccepted(index string) (bool, error) {
	name, id, valid := retiredKey(index)
	if !valid {
		return false, ErrInputUncertain
	}
	retired, err := q.openRetired(false)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer retired.dir.Close()
	shard, err := retired.read(name, retiredShardLimit)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return containsRetiredID(shard, id)
}

// Retire accepted IDs into exact fixed-size keys before deleting their index.
// A damaged or exhausted shard never silently evicts an accepted key.
func (q *inputQueue) rememberRetiredAcceptance(index string) error {
	name, id, valid := retiredKey(index)
	if !valid {
		return ErrInputUncertain
	}
	f, err := q.open(name, unix.O_RDWR|unix.O_APPEND|unix.O_CREAT)
	if err != nil {
		return err
	}
	defer f.Close()
	shard, err := readOpenInputBytes(f, retiredShardLimit, true)
	if err != nil {
		return err
	}
	present, err := containsRetiredID(shard, id)
	if err != nil {
		return err
	}
	if !present {
		if len(shard)+len(id) > retiredShardLimit {
			return ErrInputFull
		}
		if _, err := f.Write(id[:]); err != nil {
			return err
		}
	}
	return q.syncDocument(f)
}

func (q *inputQueue) retireInput(index string, r Request, receipt Receipt) error {
	retired, err := q.openRetired(true)
	if err != nil {
		return err
	}
	defer retired.dir.Close()
	if err := q.ops.syncDir(q.dir); err != nil {
		return err
	}
	if err := q.finishRetirement(retired); err != nil {
		return err
	}
	// Discover a full shard before creating the retirement checkpoint. Its
	// accepted receipts can remain in place while other records retire.
	if r.DurableRetry && receipt.State == "accepted" {
		if err := retired.rememberRetiredAcceptance(index); err != nil {
			return err
		}
	}
	tail, err := json.Marshal(inputRetirement{Index: index, Request: r, Receipt: receipt})
	if err != nil || len(tail) > retirementTailLimit {
		return ErrInputUncertain
	}
	// Exclusive ownership comes from the spool lease. A short journal write
	// leaves the original receipt and index intact and stops reclamation.
	if err := retired.createSynced(retirementTail, tail); err != nil {
		return err
	}
	return q.finishRetirement(retired)
}

func (q *inputQueue) recoverRetirement() error {
	retired, err := q.openRetired(false)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer retired.dir.Close()
	return q.finishRetirement(retired)
}

func (q *inputQueue) finishRetirement(retired *inputQueue) error {
	f, err := retired.open(retirementTail, unix.O_RDONLY)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := readOpenInputBytes(f, retirementTailLimit, true)
	var tail inputRetirement
	if err != nil || decodeInputRecord(data, &tail, inputRecordFields{names: []string{"index", "request", "receipt"}, required: 3}) != nil {
		return ErrInputUncertain
	}
	r := tail.Request
	_, _, valid := retiredKey(tail.Index)
	if !valid || r.IndexKey != "" && tail.Index != r.IndexKey+".index.json" ||
		r.Version != ProtocolVersion || r.CreatedAt <= 0 || r.Text != "" || !instancePattern.MatchString(r.ID) ||
		!validInputReceipt(tail.Receipt, r.ID) || !tail.Receipt.matchesRequest(r) ||
		tail.Receipt.State != "accepted" && tail.Receipt.State != "refused" ||
		!q.missing(r.ID+".pending.json") || !q.missing(r.ID+".claimed.json") {
		return ErrInputUncertain
	}
	if err := retired.syncDocument(f); err != nil {
		return err
	}
	index, err := q.loadIndex(tail.Index)
	hasIndex := err == nil
	if hasIndex && index != r || err != nil && !errors.Is(err, os.ErrNotExist) {
		return ErrInputUncertain
	}
	receipt, err := q.committedReceipt(r)
	hasReceipt := err == nil
	if hasReceipt && receipt != tail.Receipt || !hasReceipt && (hasIndex || !errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrInputUncertain)) {
		return ErrInputUncertain
	}
	if r.DurableRetry && tail.Receipt.State == "accepted" {
		if err := retired.rememberRetiredAcceptance(tail.Index); err != nil {
			return err
		}
	}
	if hasIndex {
		if err := q.remove(tail.Index); err != nil {
			return err
		}
	}
	// Ensure a previous index unlink is durable before removing its receipt.
	if err := q.ops.syncDir(q.dir); err != nil {
		return err
	}
	if hasReceipt {
		if err := q.remove(r.ID + ".receipt.json"); err != nil {
			return err
		}
	}
	if err := q.ops.syncDir(q.dir); err != nil {
		return err
	}
	if err := retired.remove(retirementTail); err != nil {
		return err
	}
	return retired.ops.syncDir(retired.dir)
}
