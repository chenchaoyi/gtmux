package state

import (
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
	layoutDir        = "layout-v2"
	layoutMarker     = ".storage-layout-v2"
	layoutTransition = ".storage-layout-transition.json"
	layoutFlat       = "flat"
	layoutGrouped    = "grouped"
)

var groupedStores = map[string]string{
	"active": "panes", "awaited": "panes", "bg": "panes", "cpu": "panes",
	"crashburst": "panes", "enrolled": "panes", "finished": "panes", "frame": "panes",
	"goal": "panes", "goalchanged": "panes", "helper-sessions": "panes",
	"last-finished": "panes", "sends": "panes", "tab-order": "panes", "tabalert-saved-titles": "panes",
	"terminal-lifecycle": "panes", "turn-id": "panes", "turn-outcome": "panes",
	"usagewarn": "panes", "usagewarn-gate": "panes", "waiting": "panes", "watched": "panes", "watchdog": "panes",

	"capture-requests": "clients", "native": "clients", "terminal-clients": "clients",

	"dead-pane-sweep-last": "service", "disk-hygiene-last": "service",
	"resourcewarn-gate": "service", "resourcewarn-held": "service",
	"tunnel-addresses": "service", "tunnel-watch": "service",

	"conversation-messages": "sessions", "octrans": "sessions", "participant-profiles": "sessions",
	"restore-attempts": "sessions", "restore-backstop-attempt": "sessions",
	"restore-jobs.json": "sessions", "resume": "sessions", "session-follow": "sessions",

	"account-usage": "usage", "limits.json": "usage", "limits.lock": "usage",
	"model-windows.json": "usage", "model-windows.scanned": "usage", "probe": "usage",
	"usage": "usage", "usage-daily.json": "usage", "usage-daily.json.lock": "usage",

	"hq-feed": "hq", "hq-nudge-attempts": "hq", "hq-nudge-native-batches": "hq",
	"hq-nudge-native-claims": "hq", "hq-nudges": "hq", "hq-wake-accepted": "hq",
	"hqwake": "hq", "knowledge-dream": "hq", "mine": "hq",

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
	UnmarkedGrouped bool
	Stores          []StorageLayoutStore
}

type StorageLayoutStore struct {
	Name       string
	Group      string
	Present    bool
	Unexpected bool
}

// ReadStorageLayout inventories known stores without creating any paths.
func ReadStorageLayout() (StorageLayoutStatus, error) {
	mode, err := currentLayout()
	if err != nil {
		return StorageLayoutStatus{}, err
	}
	status := StorageLayoutStatus{Mode: mode}
	transition, err := readLayoutTransaction()
	if err != nil {
		return status, err
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
	if err := inspectGroupedPaths(); err != nil {
		return status, err
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
		status.Stores = append(status.Stores, StorageLayoutStore{name, groupedStores[name], present, unexpected})
	}
	return status, nil
}

// CheckStorageLayout fences commands during an interrupted move. The app's
// desktop-facing flat paths do not participate in the transition.
func CheckStorageLayout() error {
	if _, err := os.Lstat(filepath.Join(Dir(), layoutTransition)); !errors.Is(err, os.ErrNotExist) {
		return errors.New("storage layout transition requires attention; run gtmux doctor --storage-layout")
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
	info, err := os.Lstat(filepath.Join(Dir(), layoutMarker))
	if errors.Is(err, os.ErrNotExist) {
		return layoutFlat, nil
	}
	if err != nil || checkOwnedRegular(info) != nil || info.Size() != 0 || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("invalid grouped storage layout marker")
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
	Version int
	Target  string
	Entries []layoutEntry
}

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
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || st.Nlink != 1 || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("storage layout transition requires a private file")
	}
	const limit = 32 << 10
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	if err != nil || len(data) > limit {
		return nil, errors.New("cannot read storage layout transition")
	}
	var t layoutTransaction
	if json.Unmarshal(data, &t) != nil || t.Version != 1 ||
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
	return &t, nil
}

// ChangeStorageLayout relocates known stores with an exclusive, durable
// journal. Its caller must verify all old producers have stopped.
func ChangeStorageLayout(grouped bool, checkWriters func() error) error {
	target := layoutFlat
	if grouped {
		target = layoutGrouped
	}
	lease, err := TryLease("storage-layout")
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
		return nil
	}
	if err := checkWriters(); err != nil {
		return err
	}
	if err := inspectGroupedPaths(); err != nil {
		return err
	}
	t := &layoutTransaction{Version: 1, Target: target}
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
	if err = WritePrivateFile(filepath.Join(Dir(), layoutTransition), data); err != nil {
		return err
	}
	if err := checkWriters(); err != nil {
		return err
	}
	return finishLayoutChange(t)
}

func inspectGroupedPaths() error {
	root := filepath.Join(Dir(), layoutDir)
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err := checkLayoutDir(root); err != nil {
		return err
	}
	groups := make(map[string]bool)
	for _, group := range groupedStores {
		groups[group] = true
	}
	dirs, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, dir := range dirs {
		groupPath := filepath.Join(root, dir.Name())
		if !groups[dir.Name()] {
			return fmt.Errorf("unknown grouped storage directory: %s", groupPath)
		}
		if err := checkLayoutDir(groupPath); err != nil {
			return err
		}
		stores, err := os.ReadDir(groupPath)
		if err != nil {
			return err
		}
		for _, store := range stores {
			if groupedStores[store.Name()] != dir.Name() {
				return fmt.Errorf("unknown grouped storage entry: %s", filepath.Join(groupPath, store.Name()))
			}
		}
	}
	return nil
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

func finishLayoutChange(t *layoutTransaction) error {
	if err := inspectGroupedPaths(); err != nil {
		return err
	}
	sourceMode := layoutGrouped
	if t.Target == layoutGrouped {
		sourceMode = layoutFlat
		for _, group := range layoutGroups() {
			if err := MkdirAll(filepath.Join(Dir(), layoutDir, group), 0700); err != nil {
				return err
			}
		}
		if err := SyncPrivateDirectory(filepath.Join(Dir(), layoutDir)); err != nil {
			return err
		}
		if err := SyncPrivateDirectory(Dir()); err != nil {
			return err
		}
	}
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
				return fmt.Errorf("unplanned storage object requires review: %s", path)
			}
		}
	}
	// Check all identities before advancing any further.
	for _, entry := range t.Entries {
		from, to := layoutLocations(entry.Name, sourceMode)
		if _, err := layoutEntryLocation(entry, from, to); err != nil {
			return err
		}
	}
	for _, entry := range t.Entries {
		from, to := layoutLocations(entry.Name, sourceMode)
		if err := advanceLayoutEntry(entry, from, to); err != nil {
			return err
		}
	}
	marker := filepath.Join(Dir(), layoutMarker)
	if t.Target == layoutGrouped {
		if err := WritePrivateFile(marker, nil); err != nil {
			return err
		}
	} else if err := os.Remove(marker); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if t.Target == layoutFlat {
		if err := removeEmptyLayoutDirs(); err != nil {
			return err
		}
	}
	if err := SyncPrivateDirectory(Dir()); err != nil {
		return err
	}
	if err := os.Remove(filepath.Join(Dir(), layoutTransition)); err != nil {
		return err
	}
	return SyncPrivateDirectory(Dir())
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

func removeEmptyLayoutDirs() error {
	root := filepath.Join(Dir(), layoutDir)
	groups, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, group := range groups {
		if err := os.Remove(filepath.Join(root, group.Name())); err != nil {
			return err
		}
	}
	if err := SyncPrivateDirectory(root); err != nil {
		return err
	}
	return os.Remove(root)
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
