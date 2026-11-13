package hqnudge

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	stdio "io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/chenchaoyi/gtmux/internal/events"
	"github.com/chenchaoyi/gtmux/internal/state"
)

// Intent is durable BEFORE any native call. Neither a crash nor a changed batch
// composition grants permission to repeat a possibly accepted automatic input.
// This is recovery evidence only: it is never used to authorize a new submission.
type nativeIntent struct {
	Version int           `json:"version"`
	Pane    string        `json:"pane"`
	ID      string        `json:"id"`
	Payload string        `json:"payload"`
	Outcome string        `json:"outcome"`
	Entries []nativeEntry `json:"entries"`
}
type nativeEntry struct {
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	FileID string `json:"file_id"`
}

const nativeIntentLimit = 128 << 10

func nativeIntentDir() string { return filepath.Join(state.Dir(), "hq-nudge-native-batches") }
func nativeIntentPath(r nativeIntent) string {
	b, _ := json.Marshal(struct {
		ID, Payload string
		Entries     []nativeEntry
	}{r.ID, r.Payload, r.Entries})
	h := sha256.Sum256(b)
	return filepath.Join(nativeIntentDir(), hex.EncodeToString(h[:])+".json")
}
func readNativeBytes(path string) ([]byte, error) {
	f, err := state.OpenPrivateRead(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := stdio.ReadAll(stdio.LimitReader(f, nativeIntentLimit+1))
	if err == nil && len(b) > nativeIntentLimit {
		return nil, errors.New("native wake evidence exceeds budget")
	}
	return b, err
}
func nativeFingerprint(path string) (string, string, error) {
	f, err := state.OpenPrivateRead(path)
	if err != nil {
		return "", "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", "", err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", "", errors.New("native wake inode evidence unavailable")
	}
	b, err := stdio.ReadAll(stdio.LimitReader(f, nativeIntentLimit+1))
	if err != nil {
		return "", "", err
	}
	if len(b) > nativeIntentLimit {
		return "", "", errors.New("native wake claim exceeds budget")
	}
	after, err := f.Stat()
	if err != nil {
		return "", "", err
	}
	if info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) {
		return "", "", errors.New("native wake claim changed during read")
	}
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]), fmt.Sprintf("%d:%d", uint64(stat.Dev), stat.Ino), nil
}
func writeNativeIntent(r nativeIntent) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if len(b) > nativeIntentLimit {
		return errors.New("native wake intent exceeds budget")
	}
	return state.WritePrivateFile(nativeIntentPath(r), b)
}
func prepareNativeIntent(pane, id, payload string, claims []string) (nativeIntent, error) {
	r := nativeIntent{Version: 1, Pane: pane, ID: id, Payload: payload, Outcome: "prepared"}
	for _, c := range claims {
		h, fileID, err := nativeFingerprint(c)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return r, err
		}
		r.Entries = append(r.Entries, nativeEntry{Name: filepath.Base(c), SHA256: h, FileID: fileID})
	}
	if len(r.Entries) == 0 {
		return r, errors.New("native wake claim evidence missing")
	}
	return r, writeNativeIntent(r)
}
func validNativeIntent(r nativeIntent) bool {
	if r.Version != 1 || r.Pane == "" || len(r.ID) != 7 || r.ID[0] != '#' || strings.Trim(r.ID[1:], "0123456789abcdef") != "" || r.Payload == "" || len(r.Entries) == 0 || len(r.Entries) > maxQueueEntries {
		return false
	}
	switch r.Outcome {
	case "prepared", "uncertain", "accepted", "delivered", "refused":
	default:
		return false
	}
	seen := map[string]bool{}
	for _, e := range r.Entries {
		if filepath.Base(e.Name) != e.Name || !strings.HasSuffix(e.Name, ".txt"+sendingSuffix) || len(e.SHA256) != 64 || strings.Trim(e.SHA256, "0123456789abcdef") != "" || e.FileID == "" || seen[e.Name] {
			return false
		}
		seen[e.Name] = true
	}
	return true
}

// Recovery is serialized by the existing HQ input lease. Prepared means unknown,
// including a stop before the call: retain it, never infer an unsent transaction.
func recoverNativeIntents() error {
	if err := state.MkdirAll(nativeIntentDir(), 0700); err != nil {
		return err
	}
	dir, err := os.Open(nativeIntentDir())
	if err != nil {
		return err
	}
	defer dir.Close()
	files, err := dir.ReadDir(maxQueueEntries + 1)
	if err != nil && err != stdio.EOF {
		return err
	}
	if len(files) > maxQueueEntries {
		return errors.New("native wake recovery budget exhausted")
	}
	for _, e := range files {
		if strings.HasPrefix(e.Name(), ".private-") {
			continue
		} // unpublished private writer temporary
		p := filepath.Join(nativeIntentDir(), e.Name())
		b, err := readNativeBytes(p)
		if err != nil {
			return err
		}
		var r nativeIntent
		if json.Unmarshal(b, &r) != nil || !validNativeIntent(r) || nativeIntentPath(r) != p {
			return errors.New("native wake recovery evidence invalid")
		}
		if err := settleNativeIntent(r); err != nil {
			return err
		}
	}
	return nil
}
func settleNativeIntent(r nativeIntent) error {
	// Validate every extant claim first. A replaced/corrupted leaf is not ours to
	// delete or overwrite, even when other members already reached a terminal state.
	for _, e := range r.Entries {
		p := filepath.Join(queueDir(), e.Name)
		h, fileID, err := nativeFingerprint(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return err
		}
		if fileID != e.FileID {
			return errors.New("native wake claim was replaced")
		}
		if h != e.SHA256 {
			// Refused work may have been published before an interrupted unlink.
			// The queue then owns this SAME inode and producers can update it;
			// retire only the extra claim link, never freeze or overwrite B.
			claim, claimErr := os.Lstat(p)
			queued, queueErr := os.Lstat(claimBase(p))
			published := r.Outcome == "refused" && claimErr == nil && queueErr == nil && os.SameFile(claim, queued)
			if !published {
				return errors.New("native wake claim was replaced")
			}
		}
	}
	for _, e := range r.Entries {
		p := filepath.Join(queueDir(), e.Name)
		switch r.Outcome {
		case "accepted", "delivered":
			if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		case "refused":
			if err := returnClaim(p); err != nil {
				return err
			}
		default:
			if err := parkNativeClaim(p); err != nil {
				return err
			}
		}
	}
	if r.Outcome == "prepared" {
		r.Outcome = "uncertain"
		return writeNativeIntent(r)
	}
	if r.Outcome == "uncertain" {
		return nil
	} // retain exact original payload/delivery identity
	return ignoreNativeMissing(os.Remove(nativeIntentPath(r)))
}
func ignoreNativeMissing(err error) error {
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// A hard-link publication is atomic and never replaces another producer's leaf.
// The original claim is removed only after the queue path safely owns its inode.
func returnClaim(src string) error {
	dst := claimBase(src)
	if err := os.Link(src, dst); err == nil {
		return os.Remove(src)
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	a, ea := os.Lstat(src)
	b, eb := os.Lstat(dst)
	if errors.Is(ea, os.ErrNotExist) {
		return nil
	}
	if ea != nil {
		return ea
	}
	if eb != nil {
		return eb
	}
	if os.SameFile(a, b) {
		return os.Remove(src)
	} // recovery after publication, before unlink
	name := filepath.Base(dst)
	if strings.Contains(name, ".k-") && b.Mode().IsRegular() {
		auditDroppedFile(events.DropSuperseded, src) // existing latest-keyed contract
		return os.Remove(src)
	}
	return fmt.Errorf("wake return destination already exists: %s", filepath.Base(dst))
}
func parkNativeClaim(src string) error {
	dst := claimBase(src) + nativeUncertainSuffix
	if err := os.Link(src, dst); err == nil {
		return os.Remove(src)
	} else if errors.Is(err, os.ErrNotExist) {
		return nil
	} else if !errors.Is(err, os.ErrExist) {
		return err
	}
	a, ea := os.Lstat(src)
	b, eb := os.Lstat(dst)
	if errors.Is(ea, os.ErrNotExist) {
		return nil
	}
	if ea != nil {
		return ea
	}
	if eb != nil {
		return eb
	}
	if !os.SameFile(a, b) {
		return errors.New("native uncertain destination was replaced")
	}
	return os.Remove(src)
}
