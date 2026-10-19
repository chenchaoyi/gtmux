package agentclient

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Completed retry metadata lasts 24h. Unknown claims are retained and eventually
// stop new input at the hard cap, rather than silently authorizing a duplicate.
func pruneInput(dir string, now time.Time) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	entries, err := f.ReadDir(4097)
	_ = f.Close()
	if err != nil && err != io.EOF {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".index.json") || !instancePattern.MatchString(strings.TrimSuffix(name, ".index.json")) {
			continue
		}
		var r Request
		path := filepath.Join(dir, name)
		if readOwnedJSON(path, &r, 8192) != nil || !instancePattern.MatchString(r.ID) {
			continue
		}
		age := now.UnixMilli() - r.CreatedAt
		if age < 60000 {
			continue
		}
		receipt, known := ReadReceipt(r)
		_, claimErr := os.Lstat(filepath.Join(dir, r.ID+".claimed.json"))
		if known && receipt.State == "uncertain" || !known && !os.IsNotExist(claimErr) {
			continue
		}
		if !known && r.DurableRetry && age < int64(24*time.Hour/time.Millisecond) {
			_ = os.Remove(filepath.Join(dir, r.ID+".pending.json"))
			continue
		}
		if known && age < int64(24*time.Hour/time.Millisecond) {
			continue
		}
		// Unlink only exact, owned protocol names. Links are not traversed.
		for _, suffix := range []string{".pending.json", ".receipt.json", ".claimed.json"} {
			_ = os.Remove(filepath.Join(dir, r.ID+suffix))
		}
		_ = os.Remove(path)
	}
	return nil
}
