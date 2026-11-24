package hqnudge

import (
	"encoding/json"
	"errors"
	stdio "io"
	"os"
	"path/filepath"
	"time"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/state"
)

const acceptedArchiveLimit = 2 << 20

type acceptedArchiveRecord struct {
	Key      string       `json:"key"`
	Reason   string       `json:"reason"`
	Accepted acceptedWake `json:"accepted"`
}

func acceptedArchivePath() string {
	return state.StorePath("hqwake", "accepted-archive.jsonl")
}

// Only known receiver acknowledgements may leave the active proof window.
// Both segments are private; archiving never restores the original input.
func archiveAcceptedWake(key string, a acceptedWake, reason string, now time.Time) error {
	path := acceptedArchivePath()
	lease, err := state.TryLeaseIn(filepath.Dir(path), "wake-proof-archive")
	if err != nil {
		return err
	}
	defer lease.Close()

	body, err := json.Marshal(acceptedArchiveRecord{Key: key, Reason: reason, Accepted: a})
	if err != nil {
		return err
	}
	line := append(body, '\n')
	f, err := state.OpenPrivateAppend(path)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		return errors.Join(err, f.Close())
	}
	if info.Size()+int64(len(line)) > acceptedArchiveLimit {
		if err := f.Close(); err != nil {
			return err
		}
		if err := os.Rename(path, path+".1"); err != nil {
			return err
		}
		f, err = state.OpenPrivateAppend(path)
		if err != nil {
			return err
		}
	}
	n, err := f.Write(line)
	if err == nil && n != len(line) {
		err = stdio.ErrShortWrite
	}
	if err == nil {
		err = f.Sync()
	}
	if err = errors.Join(err, f.Close()); err != nil {
		return err
	}
	if err := syncNativeDirectory(filepath.Dir(path)); err != nil {
		return err
	}
	recordIssue(issueProofArchived, now.UnixNano())
	diag.For("hq").Act("act.wake.observation.archived", "system", a.Pane, diag.Refused, "Wake receiver acknowledgement archived",
		"batch", a.Batch, "receiver_request", a.Receipt.Request, "turn", a.Receipt.Turn, "reason", reason)
	return nil
}
