package resume

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/chenchaoyi/gtmux/internal/state"
)

// NativeRestoreIncumbent distinguishes positive absence from an unreadable owner.
// ContentSHA binds the exact bytes too, including fields this Record does not use.
type NativeRestoreIncumbent struct {
	Exists     bool
	Record     Record
	ContentSHA string
}

// ReadNativeRestoreIncumbent never maps read/permission/parse failures to absence.
// After a positive lstat, even a disappearing path is an error for this observation.
func ReadNativeRestoreIncumbent(loc string) (NativeRestoreIncumbent, error) {
	var result NativeRestoreIncumbent
	path := fileFor(loc)
	before, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return result, nil
	}
	if err != nil {
		return result, fmt.Errorf("incumbent lstat: %w", err)
	}
	if !before.Mode().IsRegular() || before.Size() > 32768 {
		return result, errors.New("incumbent is not a bounded regular record")
	}
	if before.Mode().Perm()&0400 == 0 {
		return result, fmt.Errorf("incumbent owner cannot read record: %w", os.ErrPermission)
	}
	file, err := state.OpenPrivateRead(path)
	if err != nil {
		return result, fmt.Errorf("incumbent open: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) || before.Mode() != opened.Mode() ||
		before.Size() != opened.Size() || before.ModTime() != opened.ModTime() {
		return result, errors.New("incumbent changed before reading")
	}
	body, err := io.ReadAll(io.LimitReader(file, 32769))
	if err != nil {
		return result, fmt.Errorf("incumbent read: %w", err)
	}
	if len(body) > 32768 {
		return result, errors.New("incumbent exceeds read bound")
	}
	after, err := file.Stat()
	current, pathErr := os.Lstat(path)
	if err != nil || pathErr != nil || !os.SameFile(before, after) || !os.SameFile(before, current) ||
		before.Size() != after.Size() || before.Size() != current.Size() ||
		before.ModTime() != after.ModTime() || before.ModTime() != current.ModTime() ||
		before.Mode() != after.Mode() || before.Mode() != current.Mode() {
		return result, errors.New("incumbent changed while reading")
	}
	var record Record
	if json.Unmarshal(body, &record) != nil || record.Agent == "" || record.SessionID == "" {
		return result, errors.New("incumbent record is invalid")
	}
	sum := sha256.Sum256(body)
	return NativeRestoreIncumbent{Exists: true, Record: record, ContentSHA: hex.EncodeToString(sum[:])}, nil
}
