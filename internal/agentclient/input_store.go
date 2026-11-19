package agentclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/chenchaoyi/gtmux/internal/state"
)

const (
	legacyInputDir          = "terminal-input"
	activeInputDir          = "terminal-input-v2"
	inputStoreJournal       = "terminal-input-cutover.json"
	inputStoreMetadataLimit = 4096
)

var ErrInputStoreUnavailable = errors.New("terminal input store is not ready")

type inputIdentity struct {
	dev, ino string
}

func (id inputIdentity) matches(saved map[string]string) bool {
	return len(saved) == 2 && saved["dev"] == id.dev && saved["ino"] == id.ino
}

type inputStore struct {
	path              string
	directory, marker inputIdentity
	token             string
}

type inputCutover struct {
	Version           float64
	Phase             string
	Token             string
	Directory, Marker map[string]string
}

type inputMarker struct {
	Version float64
	Token   string
}

// InputDir reports the usable path without creating it. A gated layout does
// not expose either directory to a caller.
func InputDir() string {
	store, _ := resolveInputStore(state.Dir())
	return store.path
}

// ReadyForInput checks a freshly verified attachment against the current layout.
// Legacy receivers have no generation; an active layout requires an exact token.
func (a Attachment) ReadyForInput() bool {
	if !a.InputReady {
		return false
	}
	store, err := resolveInputStore(state.Dir())
	return err == nil && store.directory != (inputIdentity{}) && a.matchesInputStore(store)
}

func (a Attachment) matchesInputStore(store inputStore) bool {
	return a.Agent != "dsh" || a.InputStoreGeneration == store.token
}

// CheckInputStore is a read-only check for the generation that prepared r.
// Directory exchange does not revoke existing file descriptors. The operator
// must stop ingress and drain old writers before publishing a cutover.
func (r Request) CheckInputStore() error {
	current, err := resolveInputStore(state.Dir())
	if err != nil {
		return err
	}
	if r.store.path == "" || r.store != current {
		return fmt.Errorf("%w: generation changed", ErrInputStoreUnavailable)
	}
	return nil
}

func resolveInputStore(root string) (inputStore, error) {
	legacy := inputStore{path: filepath.Join(root, legacyInputDir)}
	active := filepath.Join(root, activeInputDir)
	if _, err := inputDirectoryIdentity(root); errors.Is(err, os.ErrNotExist) {
		return legacy, nil
	} else if err != nil {
		return inputStore{}, fmt.Errorf("%w: invalid state directory", ErrInputStoreUnavailable)
	}

	var record inputCutover
	_, err := readInputStoreDocument(filepath.Join(root, inputStoreJournal), &record, "version", "phase", "token", "directory", "marker")
	if errors.Is(err, os.ErrNotExist) {
		if _, err := os.Lstat(active); !errors.Is(err, os.ErrNotExist) {
			return inputStore{}, fmt.Errorf("%w: cutover record is missing", ErrInputStoreUnavailable)
		}
		legacy.directory, err = inputDirectoryIdentity(legacy.path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return inputStore{}, fmt.Errorf("%w: invalid legacy directory", ErrInputStoreUnavailable)
		}
		return legacy, nil
	}
	if err != nil || record.Version != 1 || record.Phase != "active" || !instancePattern.MatchString(record.Token) {
		return inputStore{}, fmt.Errorf("%w: cutover is not active", ErrInputStoreUnavailable)
	}

	directory, err := inputDirectoryIdentity(active)
	if err != nil || !directory.matches(record.Directory) {
		return inputStore{}, fmt.Errorf("%w: active directory identity changed", ErrInputStoreUnavailable)
	}
	var marker inputMarker
	id, err := readInputStoreDocument(legacy.path, &marker, "version", "token")
	if err != nil || id.dev != directory.dev || !id.matches(record.Marker) || marker.Version != 1 || marker.Token != record.Token {
		return inputStore{}, fmt.Errorf("%w: cutover marker does not match", ErrInputStoreUnavailable)
	}
	return inputStore{path: active, directory: directory, marker: id, token: record.Token}, nil
}

func inputDirectoryIdentity(path string) (inputIdentity, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return inputIdentity{}, err
	}
	identity, _, err := privateInputIdentity(info)
	if err != nil || !info.IsDir() {
		return inputIdentity{}, ErrInputStoreUnavailable
	}
	return identity, nil
}

func readInputStoreDocument(path string, value any, fields ...string) (inputIdentity, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return inputIdentity{}, err
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return inputIdentity{}, err
	}
	id, links, err := privateInputIdentity(info)
	if err != nil || !info.Mode().IsRegular() || links != 1 || info.Size() > inputStoreMetadataLimit {
		return inputIdentity{}, ErrInputStoreUnavailable
	}
	data, err := readInputSnapshot(file, info.Size())
	if err != nil {
		return inputIdentity{}, ErrInputStoreUnavailable
	}
	var shape map[string]json.RawMessage
	if checkInputJSON(data) != nil || json.Unmarshal(data, &shape) != nil || len(shape) != len(fields) {
		return inputIdentity{}, ErrInputStoreUnavailable
	}
	for _, field := range fields {
		if _, ok := shape[field]; !ok {
			return inputIdentity{}, ErrInputStoreUnavailable
		}
	}
	if err := json.Unmarshal(data, value); err != nil {
		return inputIdentity{}, ErrInputStoreUnavailable
	}
	return id, nil
}

func privateInputIdentity(info os.FileInfo) (inputIdentity, uint64, error) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Getuid() || info.Mode().Perm()&0077 != 0 {
		return inputIdentity{}, 0, ErrInputStoreUnavailable
	}
	return inputIdentity{dev: fmt.Sprint(st.Dev), ino: fmt.Sprint(st.Ino)}, uint64(st.Nlink), nil
}
