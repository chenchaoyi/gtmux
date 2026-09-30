package hq

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/chenchaoyi/gtmux/internal/diag"
	"github.com/chenchaoyi/gtmux/internal/knowledge"
	"github.com/chenchaoyi/gtmux/internal/state"
)

type migrationPreview struct {
	Encrypted          bool                       `json:"encrypted"`
	Source             string                     `json:"source"`
	Entries            []knowledge.MigrationEntry `json:"entries"`
	SensitiveCount     int                        `json:"sensitive_count"`
	HasLocal           bool                       `json:"has_local"`
	Local              string                     `json:"local,omitempty"`
	CurrentLocal       string                     `json:"current_local,omitempty"`
	CurrentLocalDigest string                     `json:"current_local_digest"`
	Tools              int                        `json:"tools"`
}
type migrationOptions struct{ Knowledge, Local, Tools, Sensitive, AllowPlain bool }
type migrationManifest struct {
	Path      string            `json:"path,omitempty"`
	V         int               `json:"v"`
	ID        string            `json:"id"`
	Source    string            `json:"source"`
	CreatedAt int64             `json:"created_at"`
	Encrypted bool              `json:"encrypted"`
	Files     map[string]string `json:"files"`
}
type migrationReceipt struct {
	OperationID string `json:"operation_id"`
	StageID     string `json:"stage_id"`
	Kind        string `json:"kind"`
	Imported    int    `json:"imported"`
	Skipped     int    `json:"skipped"`
	Committed   bool   `json:"committed"`
	Backup      string `json:"backup,omitempty"`
	Error       string `json:"error,omitempty"`
}

func migrationDir() string          { return filepath.Join(state.Dir(), "hq-migrations") }
func contentDigest(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func previewMigration(a memoryArchive, sensitive bool) (migrationPreview, error) {
	entries, n, err := knowledge.MigrationEntries(a.files[knowledge.MigrationLedgerFile].data, sensitive)
	if err != nil {
		return migrationPreview{}, err
	}
	current, err := os.ReadFile(filepath.Join(MemoryRoot(), "LOCAL.md"))
	if err != nil && !os.IsNotExist(err) {
		return migrationPreview{}, err
	}
	local, has := a.files["LOCAL.md"]
	if !utf8.Valid(local.data) || !utf8.Valid(current) || bytesContainsNUL(local.data) || bytesContainsNUL(current) {
		return migrationPreview{}, errors.New("personal requirements must be UTF-8 text without NUL bytes")
	}
	p := migrationPreview{Encrypted: a.encrypted, Source: a.digest, Entries: entries, SensitiveCount: n, HasLocal: has, Local: string(local.data), CurrentLocal: string(current), CurrentLocalDigest: contentDigest(current)}
	for name := range a.files {
		if strings.HasPrefix(name, "knowledge/tools/") {
			p.Tools++
		}
	}
	return p, nil
}

func previewRestore(a memoryArchive) (migrationPreview, error) {
	entries, n, err := knowledge.MigrationSourceEntries(a.files[knowledge.MigrationLedgerFile].data)
	if err != nil {
		return migrationPreview{}, err
	}
	_, hasLocal := a.files["LOCAL.md"]
	p := migrationPreview{Encrypted: a.encrypted, Source: a.digest, Entries: entries, SensitiveCount: n, HasLocal: hasLocal}
	for name := range a.files {
		if strings.HasPrefix(name, "knowledge/tools/") {
			p.Tools++
		}
	}
	return p, nil
}

func stageMigration(a memoryArchive, opts migrationOptions) (migrationManifest, error) {
	m := migrationManifest{V: 1, ID: diag.NewOpID(), Source: a.digest, CreatedAt: time.Now().Unix(), Encrypted: a.encrypted, Files: map[string]string{}}
	if !opts.Knowledge && !opts.Local && !opts.Tools {
		return m, errors.New("select knowledge, personal requirements, or tool attachments")
	}
	if !a.encrypted && !opts.AllowPlain {
		return m, errors.New("this archive is unencrypted; acknowledge with --allow-plain")
	}
	selected := map[string][]byte{}
	if opts.Knowledge {
		data, err := knowledge.MigrationSelection(a.files[knowledge.MigrationLedgerFile].data, opts.Sensitive)
		if err != nil {
			return m, err
		}
		if len(data) == 0 {
			return m, errors.New("archive has no selected live knowledge; legacy Markdown needs manual curation")
		}
		selected[knowledge.MigrationLedgerFile] = data
	}
	if opts.Local {
		file, ok := a.files["LOCAL.md"]
		if !ok {
			return m, errors.New("archive has no personal requirements")
		}
		selected["LOCAL.md"] = file.data
	}
	if opts.Tools {
		for name, file := range a.files {
			if strings.HasPrefix(name, "knowledge/tools/") {
				selected[name] = file.data
			}
		}
	}
	if len(selected) == 0 {
		return m, errors.New("archive has no selected content")
	}
	if err := os.MkdirAll(migrationDir(), 0700); err != nil {
		return m, err
	}
	root, err := os.MkdirTemp(migrationDir(), ".prepare-*")
	if err != nil {
		return m, err
	}
	defer os.RemoveAll(root)
	for name, data := range selected {
		dst := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0700); err != nil {
			return m, err
		}
		if err := os.WriteFile(dst, data, 0600); err != nil {
			return m, err
		}
		m.Files[name] = contentDigest(data)
	}
	b, err := json.Marshal(m)
	if err != nil {
		return m, err
	}
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), b, 0600); err != nil {
		return m, err
	}
	if err := os.Rename(root, filepath.Join(migrationDir(), m.ID)); err != nil {
		return m, err
	}
	m.Path = filepath.Join(migrationDir(), m.ID)
	return m, nil
}

var migrationIDPattern = regexp.MustCompile(`^[a-f0-9]{32}$`)
var migrationDigestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func bytesContainsNUL(b []byte) bool { return strings.IndexByte(string(b), 0) >= 0 }

func readMigration(id string) (migrationManifest, memoryArchive, error) {
	var m migrationManifest
	a := memoryArchive{files: map[string]memoryFile{}}
	if !migrationIDPattern.MatchString(id) {
		return m, a, errors.New("invalid migration ID")
	}
	root := filepath.Join(migrationDir(), id)
	b, err := readPrivateMigrationFile(root, "manifest.json", 1<<20)
	if err != nil {
		return m, a, err
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return m, a, err
	}
	if m.V != 1 || m.ID != id || !migrationDigestPattern.MatchString(m.Source) || len(m.Files) > archiveMaxEntries {
		return m, a, errors.New("unsupported migration manifest")
	}
	total := 0
	for name, digest := range m.Files {
		if name != "LOCAL.md" && name != knowledge.MigrationLedgerFile && !strings.HasPrefix(name, "knowledge/tools/") {
			return m, a, errors.New("unselected content in migration manifest")
		}
		if err := safeName(name); err != nil {
			return m, a, err
		}
		data, err := readPrivateMigrationFile(root, name, archiveFileMaxBytes)
		if err != nil {
			return m, a, err
		}
		total += len(data)
		if total > archiveMaxBytes || contentDigest(data) != digest {
			return m, a, errors.New("migration content changed; stage the original archive again")
		}
		a.files[name] = memoryFile{data, 0600}
	}
	m.Path = root
	a.encrypted, a.digest = m.Encrypted, m.Source
	return m, a, nil
}
func readPrivateMigrationFile(root, name string, limit int64) ([]byte, error) {
	// Reject symlinks in every component, not just the leaf.
	for current := root; ; {
		st, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("migration path is a symlink")
		}
		if current == migrationDir() {
			break
		}
		next := filepath.Dir(current)
		if next == current {
			return nil, errors.New("invalid migration root")
		}
		current = next
	}
	full := filepath.Join(root, filepath.FromSlash(name))
	for current := full; current != root; current = filepath.Dir(current) {
		st, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("migration content is a symlink")
		}
	}
	st, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Size() > limit {
		return nil, errors.New("migration file exceeds its limit")
	}
	return os.ReadFile(full)
}

func applyMigrationLocal(id string, a memoryArchive, expected string) (r migrationReceipt, err error) {
	err = withMigrationApplyLock(func() error { var e error; r, e = applyMigrationLocalLocked(id, a, expected); return e })
	return r, err
}

func applyMigrationLocalLocked(id string, a memoryArchive, expected string) (migrationReceipt, error) {
	r := migrationReceipt{OperationID: diag.NewOpID(), StageID: id, Kind: "local"}
	file, ok := a.files["LOCAL.md"]
	if !ok {
		return r, errors.New("personal requirements were not selected")
	}
	dst := filepath.Join(MemoryRoot(), "LOCAL.md")
	if st, err := os.Lstat(dst); err == nil && !st.Mode().IsRegular() {
		return r, errors.New("current LOCAL.md is not a regular file")
	} else if err != nil && !os.IsNotExist(err) {
		return r, err
	}
	current, err := os.ReadFile(dst)
	exists := err == nil
	if err != nil && !os.IsNotExist(err) {
		return r, err
	}
	if contentDigest(current) == contentDigest(file.data) {
		r.Skipped = 1
		return r, nil
	}
	if expected == "" || contentDigest(current) != expected {
		return r, errors.New("personal requirements changed since preview; review the current text again")
	}
	if err := os.MkdirAll(MemoryRoot(), 0700); err != nil {
		return r, err
	}
	if exists {
		r.Backup = dst + ".pre-migrate-" + r.OperationID
		backup, err := os.OpenFile(r.Backup, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return r, err
		}
		_, err = backup.Write(current)
		if err == nil {
			err = backup.Sync()
		}
		closeErr := backup.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			return r, err
		}
	}
	r.Committed, err = publishMigrationFile(dst, file.data)
	if r.Committed {
		r.Imported = 1
	}
	return r, err
}
func publishMigrationFile(dst string, data []byte) (bool, error) {
	f, err := os.CreateTemp(filepath.Dir(dst), ".migration-write-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		return false, err
	}
	if err := f.Sync(); err != nil {
		return false, err
	}
	if err := f.Close(); err != nil {
		return false, err
	}
	if err := os.Rename(f.Name(), dst); err != nil {
		return false, err
	}
	d, err := os.Open(filepath.Dir(dst))
	if err != nil {
		return true, err
	}
	defer d.Close()
	return true, d.Sync()
}

// The lock is outside the replaceable HQ home. Startup and all live imports use it.
// Refuse contention instead of leaving a menu-bar operation waiting indefinitely.
func acquireMigrationApplyLock() (func(), error) {
	if err := os.MkdirAll(state.Dir(), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(state.Dir(), ".hq-migration-apply.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if err == syscall.EWOULDBLOCK {
			return nil, errors.New("another HQ start, restore or migration is in progress; try again after it finishes")
		}
		return nil, err
	}
	return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
}
func withMigrationApplyLock(run func() error) error {
	release, err := acquireMigrationApplyLock()
	if err != nil {
		return err
	}
	defer release()
	return run()
}

func listMigrations() ([]migrationManifest, error) {
	list := make([]migrationManifest, 0)
	dirs, err := os.ReadDir(migrationDir())
	if os.IsNotExist(err) {
		return list, nil
	}
	if err != nil {
		return nil, err
	}
	for _, dir := range dirs {
		if !dir.IsDir() || !migrationIDPattern.MatchString(dir.Name()) {
			continue
		}
		m, _, err := readMigration(dir.Name())
		if err != nil {
			return nil, fmt.Errorf("migration %s: %w", dir.Name(), err)
		}
		list = append(list, m)
	}
	return list, nil
}
