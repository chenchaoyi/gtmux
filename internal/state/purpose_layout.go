package state

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const PurposeFormatCapability = "gtmux storage-format 4"

// A category describes user-visible ownership. It does not select a writer:
// the committed layout journal remains the authority for active data.
var purposeGroups = map[string]string{
	"panes": "sessions/panes", "sessions": "sessions/conversations",
	"clients": "sessions/clients", "usage": "sessions/usage",
	"hq": "hq/activity", "inbox": "hq/inbox", "backups": "backups/hq",
	"events": "diagnostics/events", "service": "services/maintenance",
}

var purposeRootStores = map[string]string{
	"uploads": "attachments/uploads", "artifacts": "attachments/previews",
	"capture-bridge": "services/screen-capture", "codex-frontends": "sessions/clients/codex-frontends",
	"terminal-clients": "sessions/clients/terminal-clients", "terminal-input": "sessions/terminal-input",
	"server-mode": "services/remote-access/server-mode", "remote-clients.json": "services/remote-access/remote-clients.json",
	"tunnel-url": "services/remote-access/tunnel-url", "status": "services/health", "logs": "diagnostics/logs",
	"limitswarn": "sessions/usage/limitswarn", "notify": "services/desktop-notifications",
}

// PurposePlacement reports existing objects without opening queue payloads,
// token files or conversation records. Never infer a completed migration from
// a destination already existing.
type PurposePlacement struct {
	Current, Destination string
	Conflict             bool
}

func InspectPurposePlacement(status StorageLayoutStatus) ([]PurposePlacement, error) {
	if status.Mode == "purpose" {
		if status.Workflow != purposeActive {
			return nil, errors.New("finish the recorded storage change first")
		}
		if err := CheckStorageLayout(); err != nil {
			return nil, err
		}
		var active []PurposePlacement
		for _, name := range purposeKeys() {
			_, to, _ := purposeLocations(name)
			if _, ok, err := workflowPathObject(filepath.Join(Dir(), to)); err != nil {
				return nil, err
			} else if ok {
				active = append(active, PurposePlacement{Current: to, Destination: to})
			}
		}
		sort.Slice(active, func(i, j int) bool { return active[i].Current < active[j].Current })
		return active, nil
	}
	if _, _, pending := status.PendingChange(); pending || status.ServicePreparation() {
		return nil, errors.New("finish the recorded storage change first")
	}
	var rows []PurposePlacement
	add := func(from, to string) error {
		_, present, err := workflowPathObject(filepath.Join(Dir(), from))
		if err != nil {
			return err
		}
		_, occupied, err := workflowPathObject(filepath.Join(Dir(), to))
		if err != nil {
			return err
		}
		if present || occupied {
			rows = append(rows, PurposePlacement{from, to, occupied})
		}
		return nil
	}
	for _, s := range status.Stores {
		if s.Unexpected {
			return nil, fmt.Errorf("inactive storage path exists for %s", s.Name)
		}
		group, ok := purposeGroups[s.Group]
		if !ok {
			return nil, fmt.Errorf("storage category missing for %s", s.Group)
		}
		from := s.Name
		if status.Mode == layoutGrouped {
			from = FormatLayoutStore(s)
		}
		if err := add(from, filepath.Join(group, s.Name)); err != nil {
			return nil, err
		}
	}
	for name, store := range workflowStores {
		dest, ok := strings.CutPrefix(store.destination, workflowLayoutDir+"/")
		if !ok {
			continue
		}
		from := name
		if status.WorkflowActive() {
			from = store.destination
		}
		if err := add(from, dest); err != nil {
			return nil, err
		}
	}
	for from, to := range purposeRootStores {
		if err := add(from, to); err != nil {
			return nil, err
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Destination < rows[j].Destination })
	return rows, nil
}

// A move stores an identity and registered key, never a caller-supplied path.
// Resolve both paths from the same registry on every retry. The source layout
// for this step is the verified workflow layout, after any older move finishes.

func purposeLocations(name string) (string, string, bool) {
	if group, ok := groupedStores[name]; ok {
		return filepath.Join(layoutDir, group, name), filepath.Join(purposeGroups[group], name), true
	}
	if store, ok := workflowStores[name]; ok {
		to, ok := strings.CutPrefix(store.destination, workflowLayoutDir+"/")
		return store.destination, to, ok
	}
	to, ok := purposeRootStores[name]
	return name, to, ok
}

func purposeKeys() []string {
	var keys []string
	for name := range groupedStores {
		keys = append(keys, name)
	}
	for name := range workflowStores {
		if _, _, ok := purposeLocations(name); ok {
			keys = append(keys, name)
		}
	}
	for name := range purposeRootStores {
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

func purposeEndpoints(key string, forward bool) (string, string, error) {
	from, to, ok := purposeLocations(key)
	if !ok {
		return "", "", fmt.Errorf("unregistered storage key %q", key)
	}
	if !forward {
		from, to = to, from
	}
	return filepath.Join(Dir(), from), filepath.Join(Dir(), to), nil
}

// An inverse plan is built from the currently active stores so anything created
// after migration returns as well. A rollback never replays the old inventory.
func planPurposeMoves(forward bool) ([]layoutEntry, error) {
	var moves []layoutEntry
	for _, key := range purposeKeys() {
		from, to, err := purposeEndpoints(key, forward)
		if err != nil {
			return nil, err
		}
		entry, present, err := workflowPathObject(from)
		if err != nil {
			return nil, err
		}
		if !forward && present && (key == "uploads" || key == "logs") {
			if _, err := inspectPurposeReference(key, entry); err != nil {
				return nil, err
			}
		} else if _, occupied, err := workflowPathObject(to); err != nil {
			return nil, err
		} else if occupied {
			return nil, fmt.Errorf("storage destination already exists: %s", to)
		}
		if !present {
			continue
		}
		if err := checkWorkflowVolume(to, entry.Dev); err != nil {
			return nil, err
		}
		entry.Name = key
		moves = append(moves, entry)
	}
	return moves, nil
}

// Validate the entire manifest, including stores absent when it was recorded,
// before moving the next object. A stale writer creating another source stops
// recovery without overwriting either copy.
func verifyPurposeMoves(moves []layoutEntry, forward, complete bool) error {
	entries := make(map[string]layoutEntry, len(moves))
	for _, m := range moves {
		if _, _, ok := purposeLocations(m.Name); !ok || m.Inode == 0 || entries[m.Name].Inode != 0 {
			return errors.New("invalid storage move manifest")
		}
		entries[m.Name] = m
	}
	for _, key := range purposeKeys() {
		from, to, err := purposeEndpoints(key, forward)
		if err != nil {
			return err
		}
		var a layoutEntry
		var src bool
		if forward && complete && (key == "uploads" || key == "logs") {
			if info, err := os.Lstat(from); err == nil && info.Mode()&os.ModeSymlink != 0 {
				if _, err := inspectPurposeReference(key, entries[key]); err != nil {
					return err
				}
			} else {
				a, src, err = workflowPathObject(from)
				if err != nil {
					return err
				}
			}
		} else {
			a, src, err = workflowPathObject(from)
		}
		if err != nil {
			return err
		}
		b, dst, err := workflowPathObject(to)
		if err != nil {
			return err
		}
		e, listed := entries[key]
		e.Name = ""
		if !listed && !src && !dst {
			continue
		}
		if listed && (dst && !src && b == e || !complete && src && !dst && a == e) {
			continue
		}
		return fmt.Errorf("storage source or destination changed: %s", key)
	}
	return nil
}

func advancePurposeMove(m layoutEntry, forward bool) error {
	from, to, err := purposeEndpoints(m.Name, forward)
	if err != nil {
		return err
	}
	// Sync all newly created ancestors, including the top-level category.
	parent := filepath.Dir(to)
	if err := MkdirAll(parent, PrivateDir); err != nil {
		return err
	}
	for dir := parent; ; dir = filepath.Dir(dir) {
		if err := checkLayoutDir(dir); err != nil {
			return err
		}
		if err := SyncPrivateDirectory(dir); err != nil {
			return err
		}
		if dir == Dir() {
			break
		}
	}
	return advanceLayoutEntry(m, from, to)
}

// The original workflow marker is renamed only at the committed switch. A
// normal command has already validated the journal and holds shared admission.
const purposeMarker = ".storage-purpose"

func purposeMarked(root string) bool {
	info, err := os.Lstat(filepath.Join(root, purposeMarker))
	return err == nil && info.IsDir()
}

func RootStorePath(name string, descendants ...string) string {
	root := Dir()
	if dest, ok := purposeRootStores[name]; ok && purposeMarked(root) {
		name = dest
	}
	return filepath.Join(append([]string{root, name}, descendants...)...)
}

// UploadReference is the spelling stored in conversations and printed by send.
// File creation always uses the physical root returned by RootStorePath.
func UploadReference(filename string) string {
	return filepath.Join(Dir(), "uploads", filepath.Base(filename))
}

var purposeStoreNames = func() map[string]string {
	names := make(map[string]string)
	for key, path := range purposeRootStores {
		names[path] = key
	}
	for key, store := range workflowStores {
		names[store.destination] = key
		names[strings.TrimPrefix(store.destination, workflowLayoutDir+"/")] = key
	}
	return names
}()

// An empty legacy scaffold alone is insufficient to offer migration on a new
// installation. Inspect names only; never read queue payloads to decide this.
func HasLegacyPurposeData(status StorageLayoutStatus) (bool, error) {
	rows, err := InspectPurposePlacement(status)
	if err != nil {
		return false, err
	}
	for _, row := range rows {
		if row.Conflict {
			return false, fmt.Errorf("storage destination exists: %s", row.Destination)
		}
		path := filepath.Join(Dir(), row.Current)
		info, err := os.Lstat(path)
		if err != nil {
			return false, err
		}
		if !info.IsDir() {
			if info.Size() > 0 {
				return true, nil
			}
			continue
		}
		if err := checkLayoutDir(path); err != nil {
			return false, err
		}
		f, err := os.Open(path)
		if err != nil {
			return false, err
		}
		names, e := f.Readdirnames(1)
		_ = f.Close()
		if e != nil && !errors.Is(e, io.EOF) {
			return false, e
		}
		if len(names) > 0 {
			return true, nil
		}
	}
	return false, nil
}
