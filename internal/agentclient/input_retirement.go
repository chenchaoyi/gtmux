package agentclient

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

const (
	stableReceiptRetention = 72 * time.Hour
	retiredInputDir        = ".retired"
	retirementTail         = "tail.json"
	retirementTailLimit    = 20 << 10
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

func (q *inputQueue) retireInput(index string, r Request, receipt Receipt) error {
	catalog, exists, err := q.archiveCatalog(false)
	if err != nil {
		return err
	}
	if !exists {
		if err := q.writeArchiveCatalog(catalog, false); err != nil {
			return err
		}
	}
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
	// Commit the accepted key before creating a destructive checkpoint.
	if r.DurableRetry && receipt.State == "accepted" {
		if err := q.rememberRetiredAcceptance(retired, index); err != nil {
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
		return errors.Join(ErrInputArchiveIntegrity, err)
	}
	defer retired.dir.Close()
	return q.finishRetirement(retired)
}

func (q *inputQueue) readRetirement() (inputRetirement, *os.File, error) {
	var tail inputRetirement
	f, err := q.open(retirementTail, unix.O_RDONLY)
	if err != nil {
		return tail, nil, err
	}
	data, err := readOpenInputBytes(f, retirementTailLimit, true)
	if err != nil || decodeInputRecord(data, &tail, inputRecordFields{names: []string{"index", "request", "receipt"}, required: 3}) != nil {
		_ = f.Close()
		return tail, nil, ErrInputArchiveIntegrity
	}
	return tail, f, nil
}

func (q *inputQueue) finishRetirement(retired *inputQueue) error {
	tail, f, err := retired.readRetirement()
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()
	r := tail.Request
	_, _, valid := archiveKey(tail.Index)
	if !valid || r.IndexKey != "" && tail.Index != r.IndexKey+".index.json" ||
		r.Version != ProtocolVersion || r.CreatedAt <= 0 || r.Text != "" || !instancePattern.MatchString(r.ID) ||
		!validInputReceipt(tail.Receipt, r.ID) || !tail.Receipt.matchesRequest(r) ||
		tail.Receipt.State != "accepted" && tail.Receipt.State != "refused" ||
		!q.missing(r.ID+".pending.json") || !q.missing(r.ID+".claimed.json") {
		return ErrInputArchiveIntegrity
	}
	if err := retired.syncDocument(f); err != nil {
		return err
	}
	index, err := q.loadIndex(tail.Index)
	hasIndex := err == nil
	if hasIndex && index != r || err != nil && !errors.Is(err, os.ErrNotExist) {
		return ErrInputArchiveIntegrity
	}
	receipt, err := q.committedReceipt(r)
	hasReceipt := err == nil
	if hasReceipt && receipt != tail.Receipt || !hasReceipt && (hasIndex || !errors.Is(err, os.ErrNotExist) || errors.Is(err, ErrInputUncertain)) {
		return ErrInputArchiveIntegrity
	}
	if r.DurableRetry && tail.Receipt.State == "accepted" {
		present, err := q.retiredAccepted(tail.Index)
		if err != nil || !present {
			return errors.Join(ErrInputArchiveIntegrity, err)
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
