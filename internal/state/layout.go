package state

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	layoutDir            = "layout-v2"
	layoutMarker         = ".storage-layout-v2"
	layoutTransition     = ".storage-layout-transition.json"
	layoutRecovery       = "layout-recovery"
	layoutFlat           = "flat"
	layoutGrouped        = "grouped"
	layoutJournalVersion = 2
)

var groupedStores = map[string]string{
	"active": "panes", "awaited": "panes", "bg": "panes", "cpu": "panes",
	"crashburst": "panes", "enrolled": "panes", "finished": "panes", "frame": "panes",
	"goal": "panes", "goalchanged": "panes", "helper-sessions": "panes",
	"last-finished": "panes", "sends": "panes", "tab-order": "panes", "tabalert-saved-titles": "panes",
	"terminal-lifecycle": "panes", "turn-id": "panes", "turn-outcome": "panes",
	"usagewarn": "panes", "usagewarn-gate": "panes", "waiting": "panes", "watched": "panes", "watchdog": "panes",

	"capture-requests": "clients", "native": "clients",

	"dead-pane-sweep-last": "service", "disk-hygiene-last": "service",
	"resourcewarn-gate": "service", "resourcewarn-held": "service",
	"tunnel-addresses": "service", "tunnel-watch": "service",

	"conversation-messages": "sessions", "octrans": "sessions", "participant-profiles": "sessions",
	"restore-attempts": "sessions", "restore-backstop-attempt": "sessions",
	"restore-closed-sessions.json": "sessions",
	"restore-jobs.json":            "sessions", "resume": "sessions", "session-follow": "sessions",

	"account-usage": "usage", "limits.json": "usage", "limits.lock": "usage",
	"model-windows.json": "usage", "model-windows.scanned": "usage", "probe": "usage",
	"usage": "usage", "usage-daily.json": "usage", "usage-daily.json.lock": "usage",

	// The HQ wake spool, locks and receipts stay at the root. Parked
	// payloads and their native claim pins can share one inode across stores.
	"hq-feed": "hq", "knowledge-dream": "hq", "mine": "hq",

	"dispatch": "inbox", "relay": "inbox", "tasks": "inbox",

	"events.jsonl": "events", "events.1.jsonl": "events", "events.seq": "events",

	"hq-export.json": "backups", "hq-snapshots": "backups",
}

// StorePath resolves a store name and optional descendants. Stores with a
// desktop, socket, input-spool or attachment path contract stay at the root.
func StorePath(name string, descendants ...string) string {
	root := Dir()
	if group, ok := groupedStores[name]; ok {
		// A malformed marker is caught by CheckStorageLayout at the CLI entry.
		// Never use absence of an individual grouped store to fall back to flat.
		if _, err := os.Lstat(filepath.Join(root, layoutMarker)); !errors.Is(err, os.ErrNotExist) {
			root = filepath.Join(root, layoutDir, group)
		}
	}
	return filepath.Join(append([]string{root, name}, descendants...)...)
}

type StorageLayoutStatus struct {
	Mode            string
	Continuing      string
	Workflow        string
	UnmarkedGrouped bool
	Stores          []StorageLayoutStore
	StagedFiles     int
	RetainedPath    string
}

type StorageLayoutStore struct {
	Name       string
	Group      string
	Present    bool
	Unexpected bool
	Progress   string
}

// ReadStorageLayout inventories known stores without creating any paths.
func ReadStorageLayout() (StorageLayoutStatus, error) {
	mode, err := currentLayout()
	if err != nil {
		return StorageLayoutStatus{}, err
	}
	status := StorageLayoutStatus{Mode: mode}
	workflow, err := readWorkflowJournal()
	if err != nil {
		return status, err
	}
	var transition *layoutTransaction
	if workflow != nil {
		status.Workflow = workflow.Phase
		if workflow.Phase == workflowGrouped {
			err = checkWorkflowActive(workflow)
		} else {
			err = verifyWorkflowMoves(workflow, false)
		}
		if err != nil {
			return status, err
		}
	} else {
		if _, found, err := workflowMarker(); err != nil || found {
			return status, errors.New("workflow layout marker exists without a verified journal")
		}
		transition, err = readLayoutTransaction()
		if err != nil {
			return status, err
		}
	}
	if transition != nil {
		status.Continuing = transition.Target
	} else if mode == layoutFlat {
		if _, err := os.Lstat(filepath.Join(Dir(), layoutDir)); err == nil {
			status.UnmarkedGrouped = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return status, err
		}
	}
	staged, err := inspectGroupedPaths(groupedRoot(), false)
	if err != nil {
		return status, err
	}
	status.StagedFiles = staged
	var locations map[string]bool
	if transition != nil {
		locations, err = verifyLayoutTransition(transition)
		if err != nil {
			return status, err
		}
		if transition.Retained != nil {
			status.RetainedPath = filepath.Join(layoutRecovery, transition.Retained.Name)
		}
	}
	for _, name := range sortedLayoutStores() {
		source, spare := layoutLocations(name, mode)
		_, present, err := layoutObject(source)
		if err != nil {
			return status, err
		}
		_, unexpected, err := layoutObject(spare)
		if err != nil {
			return status, err
		}
		store := StorageLayoutStore{Name: name, Group: groupedStores[name], Present: present, Unexpected: unexpected}
		if moved, planned := locations[name]; planned {
			store.Progress = "pending"
			if moved {
				store.Progress = "moved"
			}
		}
		status.Stores = append(status.Stores, store)
	}
	return status, nil
}

func (s StorageLayoutStatus) WorkflowActive() bool { return s.Workflow == workflowGrouped }

// PendingChange reports the direction recorded in a verified journal. A
// caller must still check the readback error and the offline preconditions.
func (s StorageLayoutStatus) PendingChange() (workflow, grouped, pending bool) {
	switch s.Workflow {
	case workflowGrouping:
		return true, true, true
	case workflowRestoring:
		return true, false, true
	}
	return false, s.Continuing == layoutGrouped, s.Continuing != ""
}

// CheckStorageLayout fences commands during an interrupted move. The app's
// desktop-facing flat paths do not participate in the transition.
func CheckStorageLayout() error {
	if _, err := os.Lstat(filepath.Join(Dir(), layoutTransition)); !errors.Is(err, os.ErrNotExist) {
		journal, err := readWorkflowJournal()
		if err != nil {
			return err
		}
		if journal == nil || journal.Phase != workflowGrouped {
			return errors.New("storage layout transition requires attention; run gtmux doctor --storage-layout")
		}
		if err := checkWorkflowActive(journal); err != nil {
			return err
		}
	} else if _, err := os.Lstat(filepath.Join(Dir(), workflowLayoutMarker)); !errors.Is(err, os.ErrNotExist) {
		return errors.New("workflow layout marker exists without its journal")
	}
	mode, err := currentLayout()
	if err != nil {
		return err
	}
	if mode == layoutFlat {
		if _, err := os.Lstat(filepath.Join(Dir(), layoutDir)); !errors.Is(err, os.ErrNotExist) {
			return errors.New("grouped state exists without a layout marker")
		}
		return nil
	}
	entries, err := os.ReadDir(Dir())
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if groupedStores[entry.Name()] != "" {
			return fmt.Errorf("a writer created a flat path during grouped storage: %s", entry.Name())
		}
	}
	return nil
}

func currentLayout() (string, error) {
	_, present, err := readLayoutMarker()
	if err != nil {
		return "", err
	}
	if !present {
		return layoutFlat, nil
	}
	if err := checkLayoutDir(filepath.Join(Dir(), layoutDir)); err != nil {
		return "", err
	}
	for _, group := range layoutGroups() {
		if err := checkLayoutDir(filepath.Join(Dir(), layoutDir, group)); err != nil {
			return "", err
		}
	}
	return layoutGrouped, nil
}

type layoutEntry struct {
	Name  string
	Dev   uint64
	Inode uint64
	Mode  uint32
}

type layoutTransaction struct {
	Version  int
	Target   string
	Entries  []layoutEntry
	Retained *layoutEntry    `json:",omitempty"`
	Marker   *layoutEntry    `json:",omitempty"`
	Scaffold *layoutScaffold `json:",omitempty"`
	proof    layoutJournalProof
}

func groupedRoot() string { return filepath.Join(Dir(), layoutDir) }

func sortedLayoutStores() []string {
	names := make([]string, 0, len(groupedStores))
	for name := range groupedStores {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func layoutGroups() []string {
	set := make(map[string]bool)
	for _, group := range groupedStores {
		set[group] = true
	}
	groups := make([]string, 0, len(set))
	for group := range set {
		groups = append(groups, group)
	}
	sort.Strings(groups)
	return groups
}

func layoutLocations(name, sourceMode string) (string, string) {
	flat := filepath.Join(Dir(), name)
	grouped := filepath.Join(Dir(), layoutDir, groupedStores[name], name)
	if sourceMode == layoutGrouped {
		return grouped, flat
	}
	return flat, grouped
}

func layoutObject(path string) (layoutEntry, bool, error) {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return layoutEntry{}, false, nil
	}
	if err != nil {
		return layoutEntry{}, false, err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Geteuid() || !info.IsDir() && (!info.Mode().IsRegular() || st.Nlink != 1) {
		return layoutEntry{}, false, fmt.Errorf("storage object requires review: %s", path)
	}
	return layoutEntry{Dev: uint64(st.Dev), Inode: uint64(st.Ino), Mode: uint32(info.Mode().Type())}, true, nil
}

func checkLayoutDir(dir string) error {
	info, err := os.Lstat(dir)
	if err != nil {
		return err
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || int(st.Uid) != os.Geteuid() || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("storage layout directory requires review: %s", dir)
	}
	return nil
}

func readLayoutTransaction() (*layoutTransaction, error) {
	file, err := OpenPrivateRead(filepath.Join(Dir(), layoutTransition))
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
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink != 1 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("storage layout transition requires a private file")
	}
	const limit = 32 << 10
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(data) > limit {
		return nil, errors.New("cannot read storage layout transition")
	}
	var t layoutTransaction
	if json.Unmarshal(data, &t) != nil || t.Version != layoutJournalVersion ||
		(t.Target != layoutFlat && t.Target != layoutGrouped) || len(t.Entries) > len(groupedStores) {
		return nil, errors.New("invalid storage layout transition")
	}
	seen := make(map[string]bool)
	for _, entry := range t.Entries {
		if groupedStores[entry.Name] == "" || seen[entry.Name] || entry.Inode == 0 ||
			(entry.Mode != 0 && entry.Mode != uint32(os.ModeDir)) {
			return nil, errors.New("invalid storage layout transition entry")
		}
		seen[entry.Name] = true
	}
	if entry := t.Retained; entry != nil && (t.Target != layoutFlat ||
		!randomLayoutSuffix(entry.Name) || entry.Inode == 0 || entry.Mode != uint32(os.ModeDir)) {
		return nil, errors.New("invalid storage layout recovery path")
	}
	if marker := t.Marker; (t.Target == layoutFlat) != (marker != nil) ||
		marker != nil && (marker.Name != "" || marker.Inode == 0 || marker.Mode != uint32(os.ModeDir)) {
		return nil, errors.New("invalid storage layout source marker")
	}
	if (t.Target == layoutFlat) != (t.Scaffold != nil) || t.Scaffold != nil && !t.Scaffold.valid() {
		return nil, errors.New("invalid storage layout rollback scaffold")
	}
	if t.Retained != nil {
		root := *t.Retained
		root.Name = ""
		if root != t.Scaffold.Root {
			return nil, errors.New("invalid storage layout recovery scaffold")
		}
	}
	t.proof = layoutJournalProof{
		file: layoutEntry{Dev: uint64(st.Dev), Inode: uint64(st.Ino)},
		hash: sha256.Sum256(data),
	}
	return &t, nil
}

// ChangeStorageLayout relocates known stores with an exclusive, durable
// journal. Its caller must verify all old producers have stopped.
func ChangeStorageLayout(grouped bool, checkWriters func() error) error {
	target := layoutFlat
	if grouped {
		target = layoutGrouped
	}
	lease, err := acquireLayoutLease(unix.LOCK_EX)
	if err != nil {
		return err
	}
	defer lease.Close()
	transition, err := readLayoutTransaction()
	if err != nil {
		return err
	}
	if transition != nil {
		if transition.Target != target {
			return fmt.Errorf("continue the interrupted %s storage transition first", transition.Target)
		}
		if err := checkWriters(); err != nil {
			return err
		}
		return finishLayoutChange(transition)
	}
	mode, err := currentLayout()
	if err != nil {
		return err
	}
	if mode == target {
		return CheckStorageLayout()
	}
	if err := checkWriters(); err != nil {
		return err
	}
	staged, err := inspectGroupedPaths(groupedRoot(), false)
	if err != nil {
		return err
	}
	t := &layoutTransaction{Version: layoutJournalVersion, Target: target}
	if target == layoutFlat {
		marker, present, err := readLayoutMarker()
		if err != nil {
			return err
		}
		if !present {
			return errors.New("grouped storage layout marker disappeared before rollback")
		}
		t.Marker = &marker
		t.Scaffold, err = readLayoutScaffold(groupedRoot())
		if err != nil {
			return err
		}
		if !t.Scaffold.valid() {
			return errors.New("cannot record the grouped storage scaffold")
		}
	}
	if staged > 0 {
		if target != layoutFlat {
			return errors.New("unmarked grouped storage contains staging files")
		}
		scaffold := t.Scaffold.Root
		scaffold.Name = rand.Text()
		t.Retained = &scaffold
	}
	for _, name := range sortedLayoutStores() {
		from, to := layoutLocations(name, mode)
		entry, present, err := layoutObject(from)
		if err != nil {
			return err
		}
		if _, occupied, err := layoutObject(to); err != nil || occupied {
			return fmt.Errorf("storage layout destination requires review: %s", to)
		}
		if present {
			if err := checkLayoutTree(from); err != nil {
				return err
			}
			entry.Name = name
			t.Entries = append(t.Entries, entry)
		}
	}
	data, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if err = publishLayoutTransaction(t, data); err != nil {
		return err
	}
	if err := checkWriters(); err != nil {
		return err
	}
	return finishLayoutChange(t)
}

// Only the exact owner-private scratch shape published by WritePrivateFile is
// eligible for retention. It is never used to reconstruct an active record.
func randomLayoutSuffix(s string) bool {
	if len(s) < 26 || len(s) > 64 {
		return false
	}
	for _, c := range s {
		if !(c >= 'A' && c <= 'Z' || c >= '2' && c <= '7') {
			return false
		}
	}
	return true
}

func inspectGroupedPaths(root string, scratchOnly bool) (int, error) {
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err := checkLayoutDir(root); err != nil {
		return 0, err
	}
	groups := make(map[string]bool)
	for _, group := range layoutGroups() {
		groups[group] = true
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return 0, err
	}
	staged := 0
	for _, dir := range dirs {
		groupPath := filepath.Join(root, dir.Name())
		if !groups[dir.Name()] {
			return 0, fmt.Errorf("unknown grouped storage directory: %s", groupPath)
		}
		if err := checkLayoutDir(groupPath); err != nil {
			return 0, err
		}
		stores, err := os.ReadDir(groupPath)
		if err != nil {
			return 0, err
		}
		for _, store := range stores {
			if !scratchOnly && groupedStores[store.Name()] == dir.Name() {
				continue
			}
			path := filepath.Join(groupPath, store.Name())
			suffix, prefixed := strings.CutPrefix(store.Name(), privateTempPrefix)
			if !prefixed || !randomLayoutSuffix(suffix) {
				return 0, fmt.Errorf("unknown grouped storage entry: %s", path)
			}
			info, err := os.Lstat(path)
			if err != nil || checkOwnedRegular(info) != nil || info.Mode().Perm() != PrivateFile || info.Sys().(*syscall.Stat_t).Nlink != 1 {
				return 0, fmt.Errorf("grouped staging entry requires review: %s", path)
			}
			staged++
		}
	}
	return staged, nil
}

func checkLayoutTree(path string) error {
	return filepath.WalkDir(path, func(name string, _ os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		_, _, err = layoutObject(name)
		return err
	})
}

func (t *layoutTransaction) sourceMode() string {
	if t.Target == layoutGrouped {
		return layoutFlat
	}
	return layoutGrouped
}

func (t *layoutTransaction) retainedPath() string {
	return filepath.Join(Dir(), layoutRecovery, t.Retained.Name)
}

// Verify every physical location before advancing. Doctor uses the same check
// so it does not suggest retrying a conflicting or unverifiable transition.
func verifyLayoutTransition(t *layoutTransaction) (map[string]bool, error) {
	if err := verifyLayoutJournal(t); err != nil {
		return nil, err
	}
	marker, marked, err := readLayoutMarker()
	if err != nil {
		return nil, err
	}
	if marked && t.Target == layoutFlat && (t.Marker == nil || marker != *t.Marker) {
		return nil, errors.New("grouped storage layout marker identity changed")
	}
	staged, err := inspectGroupedPaths(groupedRoot(), false)
	if err != nil {
		return nil, err
	}
	if staged > 0 && t.Retained == nil {
		return nil, errors.New("unplanned storage staging files require review")
	}
	if t.Retained != nil {
		if _, err := os.Lstat(filepath.Dir(t.retainedPath())); !errors.Is(err, os.ErrNotExist) {
			if err := checkLayoutDir(filepath.Dir(t.retainedPath())); err != nil {
				return nil, err
			}
		}
		archived, err := layoutEntryLocation(*t.Retained, groupedRoot(), t.retainedPath())
		if err != nil {
			return nil, err
		}
		path := groupedRoot()
		if archived {
			path = t.retainedPath()
			staged, err = inspectGroupedPaths(path, true)
			if err != nil {
				return nil, err
			}
		}
		if staged == 0 {
			return nil, errors.New("recorded storage staging files require review")
		}
		if err := t.Scaffold.verify(path, true); err != nil {
			return nil, err
		}
	} else if t.Scaffold != nil {
		if err := t.Scaffold.verify(groupedRoot(), marked); err != nil {
			return nil, err
		}
	}
	sourceMode := t.sourceMode()
	planned := make(map[string]bool)
	for _, entry := range t.Entries {
		planned[entry.Name] = true
	}
	for _, name := range sortedLayoutStores() {
		if planned[name] {
			continue
		}
		from, to := layoutLocations(name, sourceMode)
		for _, path := range []string{from, to} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				return nil, fmt.Errorf("unplanned storage object requires review: %s", path)
			}
		}
	}
	locations := make(map[string]bool, len(t.Entries))
	for _, entry := range t.Entries {
		from, to := layoutLocations(entry.Name, sourceMode)
		moved, err := layoutEntryLocation(entry, from, to)
		if err != nil {
			return nil, err
		}
		location := from
		if moved {
			location = to
		}
		if err := checkLayoutTree(location); err != nil {
			return nil, err
		}
		locations[entry.Name] = moved
	}
	if marked == (t.Target == layoutGrouped) {
		if err := requireMovedStores(t, locations); err != nil {
			return nil, err
		}
	}
	return locations, nil
}

func finishLayoutChange(t *layoutTransaction) error {
	return finishLayoutChangeWithSync(t, SyncPrivateDirectory)
}

func finishLayoutChangeWithSync(t *layoutTransaction, syncMarkerParent func(string) error) error {
	if _, err := verifyLayoutTransition(t); err != nil {
		return err
	}
	sourceMode := t.sourceMode()
	if t.Target == layoutGrouped {
		for _, group := range layoutGroups() {
			if err := MkdirAll(filepath.Join(groupedRoot(), group), 0700); err != nil {
				return err
			}
		}
		if err := SyncPrivateDirectory(groupedRoot()); err != nil {
			return err
		}
		if err := SyncPrivateDirectory(Dir()); err != nil {
			return err
		}
	}
	for _, entry := range t.Entries {
		from, to := layoutLocations(entry.Name, sourceMode)
		if err := advanceLayoutEntry(entry, from, to); err != nil {
			return err
		}
	}
	positions, err := verifyLayoutTransition(t)
	if err != nil {
		return err
	}
	if err := requireMovedStores(t, positions); err != nil {
		return err
	}
	if t.Target == layoutGrouped {
		if err := createLayoutMarker(); err != nil {
			return err
		}
	} else if err := removeLayoutMarker(*t.Marker); err != nil {
		return err
	}
	if t.Target == layoutFlat {
		// Persist marker removal before changing the scaffold. A crash must
		// not leave a visible grouped marker with a partially removed tree.
		// A retry also runs this barrier when the marker is already absent.
		if err := syncMarkerParent(Dir()); err != nil {
			return fmt.Errorf("could not sync the rollback layout marker: %w", err)
		}
		if t.Retained == nil {
			if err := removeEmptyLayoutDirs(t.Scaffold); err != nil {
				return err
			}
		} else {
			if _, err := inspectGroupedPaths(groupedRoot(), true); err != nil {
				return err
			}
			if err := advanceLayoutEntry(*t.Retained, groupedRoot(), t.retainedPath()); err != nil {
				return err
			}
		}
	}
	return completeLayoutChange(t)
}

func requireMovedStores(t *layoutTransaction, positions map[string]bool) error {
	for _, entry := range t.Entries {
		if !positions[entry.Name] {
			return fmt.Errorf("storage transition has not moved %s to its target", entry.Name)
		}
	}
	return nil
}

func completeLayoutChange(t *layoutTransaction) error {
	positions, err := verifyLayoutTransition(t)
	if err != nil {
		return err
	}
	if err := requireMovedStores(t, positions); err != nil {
		return err
	}
	if t.Retained != nil {
		archived, err := layoutEntryLocation(*t.Retained, groupedRoot(), t.retainedPath())
		if err != nil {
			return err
		}
		if !archived {
			return errors.New("interrupted staging is not yet in its recovery directory")
		}
	}
	mode, err := currentLayout()
	if err != nil {
		return err
	}
	if mode != t.Target {
		return errors.New("storage marker does not establish the target layout")
	}
	if t.Target == layoutFlat {
		if _, err := os.Lstat(groupedRoot()); !errors.Is(err, os.ErrNotExist) {
			return errors.New("grouped storage root is still present after rollback")
		}
	}
	return retireLayoutJournal(t)
}

func layoutEntryLocation(expected layoutEntry, from, to string) (bool, error) {
	expected.Name = ""
	a, here, errA := layoutObject(from)
	b, there, errB := layoutObject(to)
	if err := errors.Join(errA, errB); err != nil {
		return false, err
	}
	switch {
	case here && !there && a == expected:
		return false, nil
	case there && !here && b == expected:
		return true, nil
	default:
		return false, fmt.Errorf("storage layout identity or destination conflict: %s", from)
	}
}

func advanceLayoutEntry(entry layoutEntry, from, to string) error {
	moved, err := layoutEntryLocation(entry, from, to)
	if err != nil {
		return err
	}
	fromParent, toParent := filepath.Dir(from), filepath.Dir(to)
	if !moved {
		if err := MkdirAll(toParent, 0700); err != nil {
			return err
		}
		if err := SyncPrivateDirectory(filepath.Dir(toParent)); err != nil {
			return err
		}
		fromFD, err := privateDirectory(fromParent, false)
		if err != nil {
			return err
		}
		defer unix.Close(fromFD)
		toFD, err := privateDirectory(toParent, false)
		if err != nil {
			return err
		}
		defer unix.Close(toFD)
		if err := renameLayoutEntry(fromFD, filepath.Base(from), toFD, filepath.Base(to)); err != nil {
			return err
		}
	}
	if err := SyncPrivateDirectory(toParent); err != nil {
		return err
	}
	if err := SyncPrivateDirectory(fromParent); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// Rollback removes only empty, registered scaffold directories. A reappearing
// file, symlink or foreign directory is never a cleanup target.
func removeEmptyLayoutDirs(scaffold *layoutScaffold) error {
	parent, err := privateDirectory(Dir(), false)
	if err != nil {
		return err
	}
	defer unix.Close(parent)
	fd, err := unix.Openat(parent, layoutDir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err == unix.ENOENT {
		return nil
	}
	if err != nil {
		return fmt.Errorf("grouped storage root requires review: %w", err)
	}
	root := os.NewFile(uintptr(fd), groupedRoot())
	defer root.Close()
	if err := checkPrivateDirectory(fd, false); err != nil {
		return err
	}
	var opened unix.Stat_t
	if err := unix.Fstat(fd, &opened); err != nil {
		return err
	}
	if id, err := layoutDirectoryIdentity(opened); err != nil || id != scaffold.Root {
		return errors.New("recorded storage scaffold changed before rollback")
	}
	expected := make(map[string]layoutEntry, len(scaffold.Groups))
	for _, group := range scaffold.Groups {
		id := group
		id.Name = ""
		expected[group.Name] = id
	}
	entries, err := root.ReadDir(-1)
	if err != nil {
		return err
	}
	identities := make(map[string]layoutEntry, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		id, recorded := expected[name]
		if !entry.IsDir() || !recorded {
			return fmt.Errorf("unexpected storage object during rollback: %s", filepath.Join(groupedRoot(), name))
		}
		if current, err := layoutDirectoryAt(fd, name); err != nil || current != id {
			return fmt.Errorf("recorded storage group changed before rollback: %s", name)
		}
		identities[name] = id
	}
	for _, record := range scaffold.Groups {
		group := record.Name
		expected, listed := identities[group]
		if !listed {
			continue
		}
		if current, err := layoutDirectoryAt(fd, group); err != nil || current != expected {
			return fmt.Errorf("storage group changed during rollback: %s", group)
		}
		if err := unix.Unlinkat(fd, group, unix.AT_REMOVEDIR); err != nil {
			return fmt.Errorf("storage group requires review: %s: %w", group, err)
		}
	}
	if err := unix.Fsync(fd); err != nil {
		return err
	}
	var held, named unix.Stat_t
	if err := unix.Fstat(fd, &held); err != nil {
		return err
	}
	if err := unix.Fstatat(parent, layoutDir, &named, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return err
	}
	if held.Dev != named.Dev || held.Ino != named.Ino {
		return errors.New("grouped storage root changed during rollback")
	}
	return unix.Unlinkat(parent, layoutDir, unix.AT_REMOVEDIR)
}

// FormatLayoutStore presents relative paths without revealing record names.
func FormatLayoutStore(store StorageLayoutStore) string {
	return strings.Join([]string{layoutDir, store.Group, store.Name}, "/")
}

// StorageStoreName returns the logical name for a relative path in either layout.
func StorageStoreName(rel string) string {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) >= 3 && parts[0] == layoutDir && groupedStores[parts[2]] == parts[1] {
		return parts[2]
	}
	return parts[0]
}
