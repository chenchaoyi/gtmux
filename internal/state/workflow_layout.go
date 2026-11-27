package state

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	workflowVersion      = 3
	workflowGrouping     = "group-workflow"
	workflowGrouped      = "workflow-grouped"
	workflowRestoring    = "restore-workflow"
	workflowServicePrep  = "prepare-workflow-services"
	workflowServiceMove  = "group-workflow-with-services-held"
	workflowServiceReady = "return-workflow-services"
	workflowArchive      = "hqwake/accepted-archive.jsonl"
	workflowJournalLimit = 256 << 10
	workflowObjectLimit  = 4096
)

// Use the legacy journal pathname as a compatibility fence. Version 2 commands
// refuse admission while it exists and cannot interpret a version 3 plan.
type workflowJournal struct {
	BaseMarker   *layoutEntry `json:",omitempty"`
	HoldServices bool         `json:",omitempty"`
	PurposeToken string       `json:",omitempty"`
	Version      int
	Phase        string
	Entries      []layoutEntry `json:",omitempty"`
	Marker       *layoutEntry  `json:",omitempty"`
	proof        layoutJournalProof
}

func readWorkflowJournal() (*workflowJournal, error) {
	path := filepath.Join(Dir(), layoutTransition)
	file, err := OpenPrivateRead(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok || stat.Nlink != 1 {
		return nil, fmt.Errorf("storage layout journal requires review: %s", path)
	}
	data, err := io.ReadAll(io.LimitReader(file, workflowJournalLimit+1))
	var journal workflowJournal
	if err != nil || len(data) > workflowJournalLimit || json.Unmarshal(data, &journal) != nil {
		return nil, fmt.Errorf("storage layout journal requires review: %s", path)
	}
	if journal.Version == layoutJournalVersion {
		return nil, nil
	}
	purpose := journal.Version == purposeVersion
	if purpose {
		if err := validatePurposeJournal(&journal); err != nil {
			return nil, err
		}
	} else if journal.Version != workflowVersion || !journal.validShape() {
		return nil, fmt.Errorf("unknown workflow layout journal: %s", path)
	}
	if journal.Marker != nil && (journal.Marker.Inode == 0 || journal.Marker.Mode != uint32(os.ModeDir)) {
		return nil, fmt.Errorf("invalid workflow layout marker identity: %s", path)
	}
	if !purpose {
		seen := make(map[string]bool, len(journal.Entries))
		for _, entry := range journal.Entries {
			store, ok := workflowStoreFor(entry.Name)
			if !ok || seen[entry.Name] || entry.Inode == 0 || entry.Mode != workflowObjectType(store) {
				return nil, fmt.Errorf("unknown or duplicate workflow store: %s", entry.Name)
			}
			seen[entry.Name] = true
		}
		if !sort.SliceIsSorted(journal.Entries, func(i, j int) bool {
			return workflowNameBefore(journal.Entries[i].Name, journal.Entries[j].Name)
		}) {
			return nil, errors.New("workflow journal has an invalid move order")
		}
	}
	journal.proof = layoutJournalProof{
		file: layoutEntry{Dev: uint64(stat.Dev), Inode: uint64(stat.Ino)},
		hash: sha256.Sum256(data),
	}
	return &journal, nil
}

func (j workflowJournal) validShape() bool {
	if len(j.Entries) > workflowObjectLimit {
		return false
	}
	switch j.Phase {
	case workflowGrouping, workflowServiceMove:
		return j.Marker == nil
	case workflowRestoring:
		return j.Marker != nil
	case workflowGrouped, workflowServiceReady:
		return j.Marker != nil && len(j.Entries) == 0
	case workflowServicePrep:
		return j.Marker == nil && len(j.Entries) == 0
	default:
		return false
	}
}

func workflowObjectType(store workflowStore) uint32 {
	if store.directory {
		return uint32(os.ModeDir)
	}
	return 0
}

func workflowNameBefore(a, b string) bool {
	if a == workflowArchive || b == workflowArchive {
		return a == workflowArchive && b != workflowArchive
	}
	return a < b
}

func verifyWorkflowJournal(journal *workflowJournal) error {
	current, err := readWorkflowJournal()
	if err != nil {
		return err
	}
	if current == nil || current.proof != journal.proof {
		return errors.New("workflow layout journal identity or contents changed")
	}
	return nil
}

func publishWorkflowJournal(journal *workflowJournal, previous *workflowJournal) error {
	if previous != nil {
		if err := verifyWorkflowJournal(previous); err != nil {
			return err
		}
	}
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	if len(data) > workflowJournalLimit {
		return errors.New("workflow layout journal exceeds its scan budget")
	}
	return writePrivateFile(filepath.Join(Dir(), layoutTransition), data, func(parent int, staging string, to int, name string) error {
		file, err := layoutRegularAt(parent, staging)
		if err != nil {
			return err
		}
		if previous == nil {
			err = renameLayoutEntry(parent, staging, to, name)
		} else {
			current, statErr := layoutRegularAt(to, name)
			if statErr != nil || current != previous.proof.file {
				return errors.New("workflow layout journal identity changed before publication")
			}
			err = unix.Renameat(parent, staging, to, name)
		}
		if err == nil {
			journal.proof = layoutJournalProof{file: file, hash: sha256.Sum256(data)}
		}
		return err
	})
}

func workflowMarker() (layoutEntry, bool, error) {
	path := filepath.Join(Dir(), workflowLayoutMarker)
	entry, exists, err := layoutObject(path)
	if err == nil && exists {
		err = checkLayoutDir(path)
	}
	return entry, exists, err
}

// ChangeWorkflowLayout only runs after the caller's offline checks. The stable
// bootstrap lease covers the entire operation, including rollback and rechecks.
func ChangeWorkflowLayout(grouped bool, checkWriters func() error) error {
	lease, err := acquireLayoutLease(unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer lease.Close()
	if err := checkWriters(); err != nil {
		return err
	}
	mode, err := currentLayout()
	if err != nil {
		return err
	}
	if mode != layoutGrouped {
		return errors.New("group the base storage layout before changing workflow storage")
	}
	journal, err := readWorkflowJournal()
	if err != nil {
		return err
	}
	if journal != nil && journal.Version == purposeVersion {
		return errors.New("finish the recorded purpose layout operation with gtmux doctor --fix")
	}
	phase := workflowGrouping
	if !grouped {
		phase = workflowRestoring
	}
	prepared := grouped && journal != nil && journal.Phase == workflowServicePrep
	if journal != nil && journal.Phase != workflowGrouped && !prepared {
		if journal.Phase != phase && !(grouped && journal.Phase == workflowServiceMove) {
			return errors.New("finish the recorded workflow storage operation first")
		}
		return finishWorkflowLayout(journal, checkWriters)
	}
	if journal == nil {
		if err := CheckStorageLayout(); err != nil {
			return err
		}
	}
	if !prepared && grouped == (journal != nil) {
		return CheckStorageLayout()
	}
	if journal != nil && !prepared {
		if err := checkWorkflowActive(journal); err != nil {
			return err
		}
	}
	if prepared {
		phase = workflowServiceMove
	}
	next := &workflowJournal{Version: workflowVersion, Phase: phase}
	if journal != nil {
		next.Marker = journal.Marker
	}
	entries, err := planWorkflowMoves(grouped)
	if err != nil {
		return err
	}
	next.Entries = entries
	release, err := holdWorkflowLeases(next)
	if err != nil {
		return err
	}
	defer release()
	if err := publishWorkflowJournal(next, journal); err != nil {
		return err
	}
	return moveWorkflowLayout(next, checkWriters)
}

// SyncVerifiedStorageLayout confirms the currently visible layout and commits
// its journal directory before paused services are allowed to start.
func SyncVerifiedStorageLayout() error {
	lease, err := acquireLayoutLease(unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer lease.Close()
	journal, err := readWorkflowJournal()
	if err != nil {
		return err
	}
	if err := verifyStorageBeforeReturningServices(journal); err != nil {
		return err
	}
	return SyncPrivateDirectory(Dir())
}

func verifyStorageBeforeReturningServices(journal *workflowJournal) error {
	if journal != nil {
		switch journal.Phase {
		case purposeServicesHeld:
			if err := verifyPurposeMarker(journal); err != nil {
				return err
			}
			return checkActivePurposeRoutes(journal.PurposeToken)
		case workflowServicePrep:
			return checkWorkflowServicePreparation()
		case workflowServiceReady:
			if err := checkWorkflowActive(journal); err != nil {
				return err
			}
			return checkBaseStorageLayout()
		}
	}
	return CheckStorageLayout()
}

func checkWorkflowServicePreparation() error {
	if _, found, err := workflowMarker(); err != nil || found {
		return errors.New("the prepared service fence does not match the base layout")
	}
	if mode, err := currentLayout(); err != nil || mode != layoutGrouped {
		return errors.New("the prepared service fence needs a grouped base layout")
	}
	return checkBaseStorageLayout()
}

func finishWorkflowLayout(journal *workflowJournal, checkWriters func() error) error {
	if err := verifyWorkflowMoves(journal, false); err != nil {
		return err
	}
	for _, entry := range journal.Entries {
		from, to := workflowMovePaths(entry.Name, journal.Phase)
		moved, err := layoutEntryLocation(entry, from, to)
		if err != nil {
			return err
		}
		if moved {
			from = to
		}
		if err := checkWorkflowTree(from, entry.Dev); err != nil {
			return err
		}
	}
	release, err := holdWorkflowLeases(journal)
	if err != nil {
		return err
	}
	defer release()
	return moveWorkflowLayout(journal, checkWriters)
}

func moveWorkflowLayout(journal *workflowJournal, checkWriters func() error) error {
	if err := verifyWorkflowJournal(journal); err != nil {
		return err
	}
	if err := checkWriters(); err != nil {
		return err
	}
	if err := verifyWorkflowMoves(journal, false); err != nil {
		return err
	}
	entries := journal.Entries
	if journal.Phase == workflowRestoring {
		entries = slices.Clone(entries)
		slices.Reverse(entries)
	}
	for _, entry := range entries {
		from, to := workflowMovePaths(entry.Name, journal.Phase)
		if err := advanceLayoutEntry(entry, from, to); err != nil {
			return err
		}
	}
	if err := verifyWorkflowJournal(journal); err != nil {
		return err
	}
	if err := verifyWorkflowMoves(journal, true); err != nil {
		return err
	}
	if err := checkWriters(); err != nil {
		return err
	}
	if journal.Phase == workflowRestoring {
		return retireWorkflowLayout(journal)
	}
	path := filepath.Join(Dir(), workflowLayoutMarker)
	if err := MkdirAll(path, PrivateDir); err != nil {
		return err
	}
	if err := SyncPrivateDirectory(Dir()); err != nil {
		return err
	}
	marker, _, err := workflowMarker()
	if err != nil {
		return err
	}
	phase := workflowGrouped
	if journal.Phase == workflowServiceMove {
		phase = workflowServiceReady
	}
	active := &workflowJournal{Version: workflowVersion, Phase: phase, Marker: &marker}
	return publishWorkflowJournal(active, journal)
}

func checkWorkflowActive(journal *workflowJournal) error {
	marker, found, err := workflowMarker()
	if err != nil {
		return err
	}
	if !found || journal.Marker == nil || *journal.Marker != marker {
		return errors.New("workflow layout marker identity changed")
	}
	if mode, err := currentLayout(); err != nil || mode != layoutGrouped {
		return errors.New("workflow storage requires a verified grouped base layout")
	}
	files, err := boundedWorkflowDirectory(Dir())
	if err != nil {
		return err
	}
	for _, file := range files {
		name := file.Name()
		if store, ok := workflowStoreFor(name); ok {
			return fmt.Errorf("a writer recreated an inactive workflow path: %s (active path: %s)",
				filepath.Join(Dir(), name), filepath.Join(Dir(), store.destination))
		}
	}
	for _, store := range workflowStores {
		path := filepath.Join(Dir(), store.destination)
		entry, present, err := workflowPathObject(path)
		if err != nil {
			return err
		}
		if present && entry.Mode != workflowObjectType(store) {
			return fmt.Errorf("active workflow store type changed: %s", path)
		}
	}
	if entry, present, err := workflowPathObject(filepath.Join(Dir(), workflowLeaseDir)); err != nil {
		return err
	} else if present && entry.Mode != uint32(os.ModeDir) {
		return fmt.Errorf("global lease directory requires review: %s", workflowLeaseDir)
	}
	return nil
}

func retireWorkflowLayout(journal *workflowJournal) error {
	parent, err := privateDirectory(Dir(), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	marker, exists, err := workflowMarker()
	if err != nil || exists && marker != *journal.Marker {
		return errors.New("workflow layout marker changed during rollback")
	}
	if exists {
		if err := unix.Unlinkat(parent, workflowLayoutMarker, unix.AT_REMOVEDIR); err != nil {
			return err
		}
	}
	if err := unix.Fsync(parent); err != nil {
		return err
	}
	if err := verifyWorkflowJournal(journal); err != nil {
		return err
	}
	if err := trimWorkflowScaffold(); err != nil {
		return err
	}
	if current, err := layoutRegularAt(parent, layoutTransition); err != nil || current != journal.proof.file {
		return errors.New("workflow layout journal changed before retirement")
	}
	if err := unix.Unlinkat(parent, layoutTransition, 0); err != nil {
		return err
	}
	return unix.Fsync(parent)
}

// Remove only empty, declared workflow containers. Any retained contents
// remain in place and keep the rollback journal open for review.
func trimWorkflowScaffold() error {
	set := map[string]bool{workflowLeaseDir: true}
	for _, store := range workflowStores {
		for dir := filepath.Dir(store.destination); dir == workflowLayoutDir || strings.HasPrefix(dir, workflowLayoutDir+"/"); dir = filepath.Dir(dir) {
			set[dir] = true
		}
	}
	dirs := make([]string, 0, len(set))
	for dir := range set {
		dirs = append(dirs, dir)
	}
	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })
	for _, dir := range dirs {
		path := filepath.Join(Dir(), dir)
		if err := checkLayoutDir(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		parent := filepath.Dir(path)
		parentFD, err := privateDirectory(parent, false)
		if err != nil {
			return err
		}
		err = unix.Unlinkat(parentFD, filepath.Base(path), unix.AT_REMOVEDIR)
		if err == nil {
			err = unix.Fsync(parentFD)
		}
		unix.Close(parentFD)
		if err != nil {
			return fmt.Errorf("workflow rollback could not clear the container %s: %w", path, err)
		}
	}
	return nil
}

func workflowMovePaths(name, phase string) (string, string) {
	store, _ := workflowStoreFor(name)
	from, to := filepath.Join(Dir(), name), filepath.Join(Dir(), store.destination)
	if phase == workflowRestoring {
		return to, from
	}
	return from, to
}

func workflowNames() ([]string, error) {
	set := make(map[string]bool, len(workflowStores))
	for name := range workflowStores {
		set[name] = true
	}
	for _, dir := range []string{Dir(), filepath.Join(Dir(), workflowLeaseDir)} {
		if _, found, err := workflowPathObject(dir); err != nil {
			return nil, err
		} else if !found {
			continue
		}
		files, err := boundedWorkflowDirectory(dir)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if _, ok := workflowStoreFor(file.Name()); ok {
				set[file.Name()] = true
			}
		}
	}
	if len(set) > workflowObjectLimit {
		return nil, errors.New("too many workflow stores to inspect safely")
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return workflowNameBefore(names[i], names[j]) })
	return names, nil
}

// Check ancestors without following a symlink to an unmanaged directory.
func workflowPathObject(path string) (layoutEntry, bool, error) {
	rel, err := filepath.Rel(Dir(), path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return layoutEntry{}, false, errors.New("workflow store is outside managed storage")
	}
	parent := Dir()
	parts := strings.Split(rel, string(os.PathSeparator))
	for _, part := range parts[:len(parts)-1] {
		parent = filepath.Join(parent, part)
		if err := checkLayoutDir(parent); errors.Is(err, os.ErrNotExist) {
			return layoutEntry{}, false, nil
		} else if err != nil {
			return layoutEntry{}, false, err
		}
	}
	return layoutObject(path)
}

// WorkflowPlacement shows the registered path pair without reading payloads.
type WorkflowPlacement struct {
	Original, Grouped string
	AtGroupedPath     bool
}

func InspectWorkflowPaths() ([]WorkflowPlacement, error) {
	names, err := workflowNames()
	if err != nil {
		return nil, err
	}
	var rows []WorkflowPlacement
	for _, name := range names {
		old, grouped := workflowMovePaths(name, workflowGrouping)
		_, here, err := workflowPathObject(old)
		if err != nil {
			return nil, err
		}
		_, there, err := workflowPathObject(grouped)
		if err != nil {
			return nil, err
		}
		if here || there {
			rows = append(rows, WorkflowPlacement{Original: name, Grouped: strings.TrimPrefix(grouped, Dir()+string(os.PathSeparator)), AtGroupedPath: there})
		}
	}
	return rows, nil
}
