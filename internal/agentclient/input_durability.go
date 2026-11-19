package agentclient

import (
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

type inputDurability struct {
	syncFile   func(*os.File) error
	syncDir    func(*os.File) error
	syncParent func(*os.File) error
	link       func(int, string, int, string, int) error
	unlink     func(int, string, int) error
	renameNew  func(int, string, string) error
}

var syncInput = inputDurability{
	syncFile:   (*os.File).Sync,
	syncDir:    (*os.File).Sync,
	syncParent: (*os.File).Sync,
	link:       unix.Linkat,
	unlink:     unix.Unlinkat,
	renameNew:  renameNewInputFile,
}

// The held directory binds I/O to the directory checked by the input store.
// It does not revoke an old writer's descriptor during an operator cutover.
type inputQueue struct {
	dir *os.File
	ops inputDurability
}

func openInputQueue(store inputStore, ops inputDurability) (*inputQueue, error) {
	fd, err := unix.Open(store.path, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	dir := os.NewFile(uintptr(fd), store.path)
	info, err := dir.Stat()
	if err == nil {
		id, _, identityErr := privateInputIdentity(info)
		if identityErr != nil || !info.IsDir() || id != store.directory {
			err = ErrInputStoreUnavailable
		}
	}
	if err != nil {
		_ = dir.Close()
		return nil, err
	}
	return &inputQueue{dir: dir, ops: ops}, nil
}

func (q *inputQueue) fd() int { return int(q.dir.Fd()) }

func (q *inputQueue) open(name string, flags int) (*os.File, error) {
	fd, err := unix.Openat(q.fd(), name, flags|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0600)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), name), nil
}

func (q *inputQueue) read(name string, limit int64) ([]byte, error) {
	f, err := q.open(name, unix.O_RDONLY)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readOpenInputBytes(f, limit, true)
}

func (q *inputQueue) remove(name string) error {
	return q.ops.unlink(q.fd(), name, 0)
}

func (q *inputQueue) missing(name string) bool {
	var st unix.Stat_t
	return errors.Is(unix.Fstatat(q.fd(), name, &st, unix.AT_SYMLINK_NOFOLLOW), os.ErrNotExist)
}

func (q *inputQueue) createSynced(name string, data []byte) error {
	f, err := q.open(name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL)
	if err != nil {
		return err
	}
	_, err = f.Write(data)
	if err == nil {
		err = q.ops.syncFile(f)
	}
	return errors.Join(err, f.Close())
}

func (q *inputQueue) publishDocument(name string, data []byte) error {
	tmp := ".input-stage-" + rand.Text()
	f, err := q.open(tmp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL)
	if err != nil {
		return err
	}
	tempPresent := true
	defer func() {
		if tempPresent {
			_ = q.remove(tmp)
		}
	}()
	_, err = f.Write(data)
	if err == nil {
		err = q.ops.syncFile(f)
	}
	if err := errors.Join(err, f.Close()); err != nil {
		return err
	}
	// Link cannot replace a competing receipt. Remove our temporary name
	// before the directory barrier and before readers may accept a single link.
	if err := q.ops.link(q.fd(), tmp, q.fd(), name, 0); err != nil {
		return err
	}
	if err := q.remove(tmp); err != nil {
		return err
	}
	tempPresent = false
	return q.ops.syncDir(q.dir)
}

// Commit the ancestors that own the initially created queue path. The
// queue fd pins the first parent even if the visible path is replaced.
func (q *inputQueue) syncParents() error {
	path := q.dir.Name()
	fd := q.fd()
	var parent *os.File
	defer func() {
		if parent != nil {
			_ = parent.Close()
		}
	}()
	// state.Dir is HOME/.local/share/gtmux, plus the queue path itself.
	for range 4 {
		next, err := unix.Openat(fd, "..", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if err != nil {
			return err
		}
		if parent != nil {
			_ = parent.Close()
		}
		path = filepath.Dir(path)
		parent = os.NewFile(uintptr(next), path)
		fd = next
		if err := q.ops.syncParent(parent); err != nil {
			return err
		}
	}
	return nil
}

func (q *inputQueue) syncBoundIndex(r Request) error {
	var name string
	if r.IndexKey != "" {
		if !instancePattern.MatchString(r.IndexKey) {
			return ErrInputUncertain
		}
		name = r.IndexKey + ".index.json"
	} else {
		names, err := inputDirectoryNames(q.dir)
		if err != nil {
			return err
		}
		expected := r.persisted()
		expected.Text = ""
		for _, candidate := range names {
			key, ok := strings.CutSuffix(candidate, ".index.json")
			if !ok || !instancePattern.MatchString(key) {
				continue
			}
			if saved, err := q.loadIndex(candidate); err == nil && saved == expected {
				if name != "" {
					return ErrInputUncertain
				}
				name = candidate
			}
		}
	}
	if name == "" {
		return ErrInputUncertain
	}
	f, err := q.open(name, unix.O_RDWR)
	if err != nil {
		return err
	}
	defer f.Close()
	data, err := readOpenInputBytes(f, 8192, true)
	expected := r.persisted()
	expected.Text = ""
	var saved Request
	if err != nil || decodeInputRecord(data, &saved, requestFields) != nil || saved != expected {
		return ErrInputUncertain
	}
	if err := q.syncDocument(f); err != nil {
		return err
	}
	return q.syncParents()
}

func (q *inputQueue) openReceipt(r Request) (Receipt, *os.File, error) {
	if !instancePattern.MatchString(r.ID) {
		return Receipt{}, nil, errInvalidInputJSON
	}
	f, err := q.open(r.ID+".receipt.json", unix.O_RDWR)
	if err != nil {
		return Receipt{}, nil, err
	}
	data, err := readOpenInputBytes(f, 8192, true)
	var receipt Receipt
	if err == nil && (decodeInputRecord(data, &receipt, receiptFields) != nil || !validInputReceipt(receipt, r.ID) || !receipt.matchesRequest(r)) {
		err = errInvalidInputJSON
	}
	if err != nil {
		_ = f.Close()
		return Receipt{}, nil, err
	}
	return receipt, f, nil
}

func (q *inputQueue) syncDocument(f *os.File) error {
	if err := q.ops.syncFile(f); err != nil {
		return err
	}
	if err := q.ops.syncDir(q.dir); err != nil {
		return err
	}
	var opened, named unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &opened); err != nil {
		return err
	}
	if err := unix.Fstatat(q.fd(), f.Name(), &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if opened.Dev != named.Dev || opened.Ino != named.Ino || opened.Nlink != 1 || named.Nlink != 1 {
		return ErrInputUncertain
	}
	return nil
}

func (q *inputQueue) committedReceipt(r Request) (Receipt, error) {
	receipt, f, err := q.openReceipt(r)
	if err != nil {
		return Receipt{}, err
	}
	defer f.Close()
	if err := q.syncDocument(f); err != nil {
		// The receipt was opened, so ENOENT during revalidation cannot be
		// interpreted as a receipt that never existed.
		return Receipt{}, errors.Join(ErrInputUncertain, err)
	}
	return receipt, nil
}
