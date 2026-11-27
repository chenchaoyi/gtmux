package state

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"golang.org/x/sys/unix"
)

// v1.0.203 DSH reads this path independently of CLI admission and rejects an
// unsupported version. Keep it fenced for the entire offline change, including
// rollback. Never replace an unrelated or unrecognized cutover document.
const purposeInputRecord = "terminal-input-cutover.json"
const purposeInputMarker = "services/maintenance/terminal-input-generation.json"

func purposeInputToken() (string, error) {
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}

type inputGeneration struct {
	Version int    `json:"version"`
	Token   string `json:"token"`
}

type inputGenerationRecord struct {
	Version   int               `json:"version"`
	Phase     string            `json:"phase"`
	Token     string            `json:"token"`
	Directory map[string]string `json:"directory"`
	Marker    map[string]string `json:"marker"`
}

func inputRecordBytes(phase, token string, dir, marker layoutEntry) []byte {
	identity := func(id layoutEntry) map[string]string {
		return map[string]string{"dev": strconv.FormatUint(id.Dev, 10), "ino": strconv.FormatUint(id.Inode, 10)}
	}
	data, _ := json.Marshal(inputGenerationRecord{2, phase, token, identity(dir), identity(marker)})
	return data
}

func purposeDocument(path string) ([]byte, layoutEntry, error) {
	id, ok, err := workflowPathObject(path)
	if err != nil {
		return nil, id, err
	}
	if !ok {
		return nil, id, os.ErrNotExist
	}
	if id.Mode != 0 {
		return nil, id, fmt.Errorf("unexpected document type at %s", path)
	}
	f, err := OpenPrivateRead(path)
	if err != nil {
		return nil, id, err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return nil, id, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink != 1 || uint64(st.Dev) != id.Dev || uint64(st.Ino) != id.Inode {
		return nil, id, errors.New("storage document changed during read")
	}
	if info.Mode().Perm() != 0600 || info.Size() > 4096 {
		return nil, id, fmt.Errorf("unrecognized storage document: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(f, 4097))
	if len(data) > 4096 {
		return nil, id, errors.New("storage document exceeds its bound")
	}
	return data, id, err
}

// Only the expected previous bytes may be replaced. A retry that already sees
// next must still sync the parent after a previously failed publication barrier.
func publishPurposeDocument(rel string, previous, next []byte) error {
	path := filepath.Join(Dir(), rel)
	current, id, err := purposeDocument(path)
	absent := errors.Is(err, os.ErrNotExist)
	if err != nil && !absent {
		return err
	}
	if !absent && bytes.Equal(current, next) {
		return SyncPrivateDirectory(filepath.Dir(path))
	}
	if absent != (previous == nil) || !bytes.Equal(current, previous) {
		return fmt.Errorf("storage document changed: %s", rel)
	}
	return writePrivateFile(path, next, func(parent int, staging string, to int, name string) error {
		if absent {
			return renameLayoutEntry(parent, staging, to, name)
		}
		now, err := layoutRegularAt(to, name)
		if err != nil {
			return err
		}
		if now != id {
			return errors.New("storage document identity changed before replacement")
		}
		return unix.Renameat(parent, staging, to, name)
	})
}

func activatePurposeInput(token string) ([]byte, error) {
	if len(token) != 32 {
		return nil, errors.New("invalid input generation")
	}
	empty := layoutEntry{}
	paused := inputRecordBytes("paused", token, empty, empty)
	data, _ := json.Marshal(inputGeneration{2, token})
	if err := publishPurposeDocument(purposeInputMarker, nil, data); err != nil {
		return nil, err
	}
	for _, dir := range []string{"services/maintenance", "services", "."} {
		if err := SyncPrivateDirectory(filepath.Join(Dir(), dir)); err != nil {
			return nil, err
		}
	}
	_, marker, err := purposeDocument(filepath.Join(Dir(), purposeInputMarker))
	if err != nil {
		return nil, err
	}
	_, path, _ := purposeEndpoints("terminal-input", true)
	queue, exists, err := workflowPathObject(path)
	if err != nil {
		return nil, err
	}
	if !exists || queue.Mode != uint32(os.ModeDir) {
		return nil, errors.New("verified terminal input directory is missing")
	}
	active := inputRecordBytes("active", token, queue, marker)
	return active, publishPurposeDocument(purposeInputRecord, paused, active)
}

func removePurposeDocument(rel string, wanted []byte) error {
	path := filepath.Join(Dir(), rel)
	data, id, err := purposeDocument(path)
	if errors.Is(err, os.ErrNotExist) {
		for dir := filepath.Dir(path); ; dir = filepath.Dir(dir) {
			e := SyncPrivateDirectory(dir)
			if !errors.Is(e, os.ErrNotExist) || dir == Dir() {
				return e
			}
		}
	}
	if err != nil {
		return err
	}
	if !bytes.Equal(data, wanted) {
		return fmt.Errorf("storage document has changed: %s", rel)
	}
	parent, err := privateDirectory(filepath.Dir(path), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	name := filepath.Base(path)
	now, err := layoutRegularAt(parent, name)
	if err != nil {
		return err
	}
	if now != id {
		return errors.New("storage document identity changed before removal")
	}
	if err := unix.Unlinkat(parent, name, 0); err != nil {
		return err
	}
	return unix.Fsync(parent)
}

func verifyActivePurposeInput(token string) error {
	queue, present, err := workflowPathObject(filepath.Join(Dir(), "sessions", "terminal-input"))
	if err != nil {
		return err
	}
	if !present || queue.Mode != uint32(os.ModeDir) {
		return errors.New("expected private terminal input directory")
	}
	data, id, err := purposeDocument(filepath.Join(Dir(), purposeInputMarker))
	if err != nil {
		return err
	}
	want, _ := json.Marshal(inputGeneration{2, token})
	if !bytes.Equal(data, want) {
		return errors.New("terminal input generation does not match this layout")
	}
	record, _, err := purposeDocument(filepath.Join(Dir(), purposeInputRecord))
	if err != nil {
		return err
	}
	if !bytes.Equal(record, inputRecordBytes("active", token, queue, id)) {
		return errors.New("terminal input record does not match the active directory and marker")
	}
	return nil
}
