package state

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

// WorkflowMigrationHasData distinguishes old workflow records from scaffolding
// created by a new installation. Layout validity is checked by the caller.
func WorkflowMigrationHasData() (bool, error) {
	paths, err := InspectWorkflowPaths()
	if err != nil {
		return false, err
	}
	for _, path := range paths {
		if path.AtGroupedPath {
			return false, fmt.Errorf("destination already exists: %s", path.Grouped)
		}
	}
	for _, path := range paths {
		store, _ := workflowStoreFor(path.Original)
		// A standalone lease is runtime scaffolding, not a workflow record.
		if !store.directory && path.Original != workflowArchive {
			continue
		}
		old := filepath.Join(Dir(), path.Original)
		entry, present, err := workflowPathObject(old)
		if err != nil {
			return false, err
		}
		if !present || entry.Mode != workflowObjectType(store) {
			return false, fmt.Errorf("workflow store changed during inspection: %s", old)
		}
		hasData := false
		err = scanWorkflowTree(old, entry.Dev, func(info fs.FileInfo) error {
			if info.Size() > 0 {
				hasData = true
				return fs.SkipAll
			}
			return nil
		})
		if err != nil || hasData {
			return hasData, err
		}
	}
	return false, nil
}

func planWorkflowMoves(grouped bool) ([]layoutEntry, error) {
	phase := workflowGrouping
	if !grouped {
		phase = workflowRestoring
	}
	names, err := workflowNames()
	if err != nil {
		return nil, err
	}
	var entries []layoutEntry
	for _, name := range names {
		from, to := workflowMovePaths(name, phase)
		entry, exists, err := workflowPathObject(from)
		if err != nil {
			return nil, err
		}
		_, occupied, err := workflowPathObject(to)
		if err != nil {
			return nil, err
		}
		if occupied {
			return nil, fmt.Errorf("workflow destination already exists: %s", to)
		}
		if !exists {
			continue
		}
		store, _ := workflowStoreFor(name)
		if entry.Mode != workflowObjectType(store) {
			return nil, fmt.Errorf("workflow store type changed: %s", from)
		}
		if err := checkWorkflowVolume(to, entry.Dev); err != nil {
			return nil, err
		}
		if err := checkWorkflowTree(from, entry.Dev); err != nil {
			return nil, err
		}
		entry.Name = name
		entries = append(entries, entry)
	}
	return entries, nil
}

// Wake payloads and their claim pins can legitimately link to one inode.
// Whole-directory renames preserve both names without rewriting the files.
func checkWorkflowTree(root string, device uint64) error {
	return scanWorkflowTree(root, device, nil)
}

func scanWorkflowTree(root string, device uint64, visitFile func(fs.FileInfo) error) error {
	const entryLimit = 64 * 1024
	count := 0
	return filepath.WalkDir(root, func(path string, object fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		count++
		if count > entryLimit {
			return fmt.Errorf("workflow directory exceeds its scan budget: %s", root)
		}
		info, err := object.Info()
		if err != nil {
			return err
		}
		stat, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(stat.Dev) != device {
			return fmt.Errorf("workflow object is on another volume: %s", path)
		}
		if object.IsDir() {
			return checkLayoutDir(path)
		}
		if err := checkOwnedRegular(info); err != nil || info.Mode().Perm()&0077 != 0 {
			return fmt.Errorf("workflow file requires review: %s", path)
		}
		if visitFile != nil {
			return visitFile(info)
		}
		return nil
	})
}

func checkWorkflowVolume(destination string, device uint64) error {
	for dir := filepath.Dir(destination); ; dir = filepath.Dir(dir) {
		entry, present, err := workflowPathObject(dir)
		if err != nil {
			return err
		}
		if present {
			if entry.Dev != device || entry.Mode != uint32(os.ModeDir) {
				return fmt.Errorf("workflow destination parent is not a directory on the same volume: %s", dir)
			}
			return nil
		}
		if dir == Dir() {
			return fmt.Errorf("workflow data root is missing: %s", dir)
		}
	}
}

func verifyWorkflowMoves(journal *workflowJournal, complete bool) error {
	marker, marked, err := workflowMarker()
	if err != nil {
		return err
	}
	if journal.Phase == workflowRestoring && marked && marker != *journal.Marker {
		return errors.New("workflow layout marker changed during rollback")
	}
	allMoved := complete || (journal.Phase == workflowGrouping || journal.Phase == workflowServiceMove) && marked ||
		journal.Phase == workflowRestoring && !marked
	planned := make(map[string]layoutEntry, len(journal.Entries))
	for _, entry := range journal.Entries {
		planned[entry.Name] = entry
	}
	names, err := workflowNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		from, to := workflowMovePaths(name, journal.Phase)
		_, here, err := workflowPathObject(from)
		if err != nil {
			return err
		}
		_, there, err := workflowPathObject(to)
		if err != nil {
			return err
		}
		entry, exists := planned[name]
		if !exists {
			if here || there {
				return fmt.Errorf("unrecorded workflow store appeared: %s or %s", from, to)
			}
			continue
		}
		delete(planned, name)
		moved, err := layoutEntryLocation(entry, from, to)
		if err != nil {
			return fmt.Errorf("%w (other path: %s)", err, to)
		}
		if allMoved && !moved {
			return fmt.Errorf("workflow store has not reached its recorded path: %s", to)
		}
		if !moved {
			if err := checkWorkflowVolume(to, entry.Dev); err != nil {
				return err
			}
		}
	}
	// A dynamic lease can disappear from both directories. The journal, not
	// the directory listing, decides whether that identity is still required.
	for name := range planned {
		from, to := workflowMovePaths(name, journal.Phase)
		return fmt.Errorf("workflow lease is missing from %s and %s", from, to)
	}
	return nil
}

func holdWorkflowLeases(journal *workflowJournal) (func(), error) {
	var files []*os.File
	release := func() {
		for _, file := range files {
			_ = file.Close()
		}
	}
	for _, entry := range journal.Entries {
		if !strings.HasSuffix(entry.Name, ".lock") {
			continue
		}
		from, to := workflowMovePaths(entry.Name, journal.Phase)
		moved, err := layoutEntryLocation(entry, from, to)
		if err != nil {
			release()
			return nil, err
		}
		if moved {
			from = to
		}
		file, err := openWorkflowLease(from, entry)
		if err != nil {
			release()
			return nil, err
		}
		files = append(files, file)
	}
	return release, nil
}

func openWorkflowLease(path string, expected layoutEntry) (*os.File, error) {
	file, err := OpenPrivateRead(path)
	if err != nil {
		return nil, err
	}
	var stat syscall.Stat_t
	if err := syscall.Fstat(int(file.Fd()), &stat); err != nil ||
		uint64(stat.Dev) != expected.Dev || uint64(stat.Ino) != expected.Inode || stat.Nlink != 1 {
		file.Close()
		return nil, fmt.Errorf("workflow lease identity changed: %s", path)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		file.Close()
		if errors.Is(err, unix.EWOULDBLOCK) {
			return nil, fmt.Errorf("workflow lease is held: %s: %w", path, ErrLeaseHeld)
		}
		return nil, err
	}
	return file, nil
}

func boundedWorkflowDirectory(dir string) ([]os.DirEntry, error) {
	file, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	entries, err := file.ReadDir(workflowObjectLimit + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, err
	}
	if len(entries) > workflowObjectLimit {
		return nil, fmt.Errorf("too many directory entries to inspect safely: %s", dir)
	}
	return entries, nil
}
