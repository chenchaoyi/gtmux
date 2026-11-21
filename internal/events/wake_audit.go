package events

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// Only the wake observer publishes a deterministic delivery audit ID. Check
// the exact retained record on both journal segments before attempting another
// append. Parsing a record is only necessary when its raw key matches.
func retainedWakeAudit(id, pane, summary string) (bool, error) {
	key, _ := json.Marshal(id)
	needle := append([]byte(`"op_id":`), key...)
	for _, path := range []string{Path(), rotatedPath()} {
		f, err := state.OpenPrivateRead(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, err
		}
		found, scanErr := scanWakeAudit(f, needle, id, pane, summary)
		closeErr := f.Close()
		if err := errors.Join(scanErr, closeErr); err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func scanWakeAudit(f *os.File, needle []byte, id, pane, summary string) (bool, error) {
	scan := bufio.NewScanner(f)
	scan.Buffer(make([]byte, 64<<10), 1<<20)
	for scan.Scan() {
		line := scan.Bytes()
		if !bytes.Contains(line, needle) {
			continue
		}
		var r Record
		if err := json.Unmarshal(line, &r); err != nil {
			return false, err
		}
		if r.OpID != id {
			continue
		}
		if r.Event != AuditEventWakeDelivered || r.Pane != pane || r.Summary != summary {
			return false, errors.New("wake audit identity refers to different evidence")
		}
		return true, nil
	}
	return false, scan.Err()
}
